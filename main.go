package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cta.epfl.ch/mr-feature-controller/git"
	"cta.epfl.ch/mr-feature-controller/utils"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/exp/slices"
)

func loadCurrentEnv(repo *git.Repository) ([]int, error) {
	files, err := os.ReadDir(filepath.Join(repo.Folder, "apps/esap/mr"))
	if err != nil {
		log.Printf("Unable to read mr folder: %s\n", err)
		return nil, err
	}

	existingEnvIds := []int{}
	for _, file := range files {

		if file.IsDir() && strings.HasPrefix(file.Name(), "mr-") {
			id, err := strconv.Atoi(strings.TrimPrefix(file.Name(), "mr-"))
			if err != nil {
				existingEnvIds = append(existingEnvIds, id)
			}
		}
	}
	return existingEnvIds, nil
}

func spawnNewEnv(repo *git.Repository, newMergeRequests []*gitlab.MergeRequest, envPrefix string) {
	repo.Pull()

	for _, mergeRequest := range newMergeRequests {
		// Namespace
		mrId := strconv.Itoa(mergeRequest.ID)
		log.Printf("Generate helm chart for MR %s", mrId)

		base := filepath.Join(repo.Folder, "apps/esap/mr")
		reference := filepath.Join(base, "reference")
		cloned := filepath.Join(base, "mr-"+strconv.Itoa(mergeRequest.ID))

		if _, err := os.Stat(cloned); os.IsNotExist(err) {
			cmd := exec.Command("cp", "--recursive", reference, cloned)
			err := cmd.Run()
			if err != nil {
				log.Println(reference)
				log.Println(cloned)
				log.Fatalf("Error while duplicating reference folder: %s", err)
			}

			files := []string{
				filepath.Join(cloned, "esap-values.yaml"),
				filepath.Join(cloned, "namespace.yaml"),
				filepath.Join(cloned, "kustomization.yaml"),
				filepath.Join(cloned, "django-secret-key-secret.yaml"),
				filepath.Join(cloned, "gitlab-ctao-secret.yaml"),
			}

			searchValue := "esap-mr"
			replaceValue := "esap-mr-" + mrId
			for _, file := range files {
				utils.ReplaceInFile(file, searchValue, replaceValue)
			}

			utils.ReplaceInFile(filepath.Join(base, "kustomization.yaml"), "resources:", "resources:\n  - mr-"+mrId+"/kustomization.yaml")
			log.Printf("Create new env: %s\n", "mr-"+mrId)
		}
	}

	err := repo.AddAll()
	if err != nil {
		log.Fatalf("Add all error: %s", err)
	}
	err = repo.Commit("[MR Controller] spawn new envs")
	if err != nil {
		log.Printf("Commit error: %s", err)
		time.Sleep(30 * time.Minute)
		log.Fatalf("Commit error: %s", err)
	}
	err = repo.Push()
	if err != nil {
		log.Fatalf("Push error: %s", err)
	}
}

func reapOldEnv(repo *git.Repository, envIdsToDrop []int, envPrefix string) {
	repo.Pull()

	for _, envId := range envIdsToDrop {
		// Namespace
		mrId := strconv.Itoa(envId)
		base := filepath.Join(repo.Folder, "apps/esap/mr")
		path := filepath.Join(base, "mr-"+mrId)

		err := os.RemoveAll(path)
		if err != nil {
			log.Printf("Error while ripping env: %s - %s\n", "mr-"+mrId, err)
		} else {
			log.Printf("Reap outdated env: %s\n", "mr-"+mrId)
		}
		utils.ReplaceInFile(base+"kustomization.yaml", "  - mr-"+mrId+"/kustomization.yaml\n", "")
	}

	repo.AddAll()
	repo.Commit("[MR Controller] reaped old envs")
	repo.Push()
}

func loop(gitlabApi *gitlab.Client, fluxRepo *git.Repository) {
	// TODO: implement
	// 0. Load existing environements -> OK
	// 1. Get open MR -> OK
	// 2. Detect closed MR
	// 2a. Delete environement
	// 3. Detect opened MR
	// 3a. Generate Helm Charts
	// 4. Update edited MR
	// 4a. Update environement
	// 5. Messages on GitLab MR

	// TODO: Extract in option struct
	targetBranch := os.Getenv("TARGET_BRANCH")
	projectId := os.Getenv("GITLAB_PROJECT_ID")
	envPrefix := os.Getenv("ENV_PREFIX")

	existingEnvIds, err := loadCurrentEnv(fluxRepo)
	if err != nil {
		log.Fatalf("Unable to load current env from flux repository: %s", err)
	}
	log.Printf("Loaded %s envs : %v\n", strconv.Itoa(len(existingEnvIds)), existingEnvIds)

	openedState := "opened"
	openMergeRequests, _, err := gitlabApi.MergeRequests.ListProjectMergeRequests(projectId, &gitlab.ListProjectMergeRequestsOptions{
		TargetBranch: &targetBranch,
		State:        &openedState,
	})
	if err != nil {
		// TODO: Manage error
		log.Printf("Unable to list project MR: %s\n", err)
	}

	// Identify new MR
	var openMergeRequestIds []int
	newMergeRequests := []*gitlab.MergeRequest{}
	for _, mergeRequest := range openMergeRequests {
		openMergeRequestIds = append(openMergeRequestIds, mergeRequest.ID)
		if !slices.Contains(existingEnvIds, mergeRequest.ID) {
			newMergeRequests = append(newMergeRequests, mergeRequest)
		}
	}
	log.Printf("Loaded %s open MR: %v\n", strconv.Itoa(len(openMergeRequests)), openMergeRequestIds)

	spawnNewEnv(fluxRepo, newMergeRequests, envPrefix)

	// Identify env to reap
	envIdsToDrop := []int{}
	for _, id := range existingEnvIds {
		if !slices.Contains(openMergeRequestIds, id) {
			envIdsToDrop = append(envIdsToDrop, id)
		}
	}
	reapOldEnv(fluxRepo, envIdsToDrop, envPrefix)

	// TODO: Identify env top update
}

func main() {
	log.Println("Starting server")

	utils.InitSshConfig()

	// Flux repository
	repository := os.Getenv("FLUX_REPOSITORY")
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	repo, err := git.NewGit(dir, repository)
	if err != nil {
		log.Printf("Error while initialising main flux config repository: %s", err)
		time.Sleep(60 * time.Minute)
	}

	// Watched repository
	gitlabUrl := os.Getenv("GITLAB_URL")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	git, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(gitlabUrl))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ticker := time.NewTicker(2 * time.Minute)
	quit := make(chan bool)
	go func() {
		loop(git, repo)
		for {
			log.Println("Loop start")
			select {
			case <-ticker.C:
				loop(git, repo)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	// create a deadline to wait for.
	_, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	quit <- true

	log.Println("Shutting down")
	os.Exit(0)
}

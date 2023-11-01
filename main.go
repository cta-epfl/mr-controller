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

type MrDeployStatus string

const (
	NotDeployed     MrDeployStatus = "NOT_DEPLOYED"
	UpToDate        MrDeployStatus = "UP_TO_DATE"
	UpdateAvailable MrDeployStatus = "UPDATE_AVAILABLE"
	Pending         MrDeployStatus = "PENDING"
	Desynchronized  MrDeployStatus = "DESYNCHRONIZED"
)

// type PipelineStatus int64

// const (
// 	Pending PipelineStatus = iota
// 	Failed
// 	Success
// )

type AppConfig struct {
	pid          string
	targetBranch string
	envPrefix    string
}

type App struct {
	config *AppConfig

	repo   *git.Repository
	gitlab *gitlab.Client
}

func NewApp(repo *git.Repository, gitlabApi *gitlab.Client, config *AppConfig) *App {
	return &App{
		config: config,
		repo:   repo,
		gitlab: gitlabApi,
	}
}

func (app *App) loadCurrentEnv() ([]int, error) {
	app.repo.Pull()

	files, err := os.ReadDir(filepath.Join(app.repo.Folder, "apps/esap/mr/"))
	if err != nil {
		log.Printf("Unable to read mr folder: %s\n", err)
		return nil, err
	}

	existingEnvIds := []int{}
	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), "mr-") {
			id, err := strconv.Atoi(strings.TrimPrefix(file.Name(), "mr-"))
			if err == nil {
				existingEnvIds = append(existingEnvIds, id)
			}
		}
	}
	return existingEnvIds, nil
}

func (app *App) spawnNewEnv(newMergeRequests []*gitlab.MergeRequest, envPrefix string) {
	app.repo.Pull()

	for _, mergeRequest := range newMergeRequests {
		commits, _, err := app.gitlab.MergeRequests.GetMergeRequestCommits(app.config.pid, mergeRequest.IID, &gitlab.GetMergeRequestCommitsOptions{PerPage: 1})
		if err != nil || len(commits) == 0 {
			log.Printf("No commit identified for MR %d : %s", mergeRequest.IID, err)
			continue
		}
		tag := strconv.Itoa(int(commits[0].CommittedDate.Unix()))

		// Namespace
		mrId := strconv.Itoa(mergeRequest.IID)
		log.Printf("Generate helm chart for MR %s", mrId)

		base := filepath.Join(app.repo.Folder, "apps/esap/mr")
		reference := filepath.Join(base, "reference")
		cloned := filepath.Join(base, "mr-"+strconv.Itoa(mergeRequest.IID))

		if _, err := os.Stat(cloned); os.IsNotExist(err) {
			cmd := exec.Command("cp", "--recursive", reference, cloned)
			err := cmd.Run()

			if err != nil {
				log.Println(reference)
				log.Println(cloned)
				log.Fatalf("Error while duplicating reference folder: %s", err)
			}

			files, err := os.ReadDir(cloned)
			if err != nil {
				log.Fatalf("Error while listing duplicated files: %s", err)
			}

			searchValue := "esap-mr-{id}"
			replaceValue := "esap-mr-" + mrId
			for _, file := range files {
				utils.ReplaceInFile(filepath.Join(cloned, file.Name()), searchValue, replaceValue)
			}

			replaceText := "resources:\n  - ./mr-" + mrId
			replaced := utils.ReplaceInFile(filepath.Join(base, "kustomization.yaml"), "resources:", replaceText)
			if !replaced {
				f, err := os.OpenFile(filepath.Join(base, "kustomization.yaml"), os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					log.Println(err)
				}
				defer f.Close()
				if _, err := f.WriteString(replaceText); err != nil {
					log.Println(err)
				}
			}
			log.Printf("Create new env: %s\n", "mr-"+mrId)

			utils.ReplaceInFile(filepath.Join(cloned, "esap-values.yaml"), "{image-tag}", tag)
		}
	}

	err := app.repo.AddAll()
	if err != nil {
		log.Fatalf("Add all error: %s", err)
	}
	err = app.repo.Commit("[MR Controller] spawn new envs")
	if err != nil {
		log.Printf("Commit error: %s", err)
		time.Sleep(30 * time.Minute)
		log.Fatalf("Commit error: %s", err)
	}
	err = app.repo.Push()
	if err != nil {
		log.Fatalf("Push error: %s", err)
	}
}

func (app *App) reapOldEnv(envIdsToDrop []int, envPrefix string) {
	app.repo.Pull()

	base := filepath.Join(app.repo.Folder, "apps/esap/mr")

	for _, envId := range envIdsToDrop {
		// Namespace
		mrId := strconv.Itoa(envId)
		path := filepath.Join(base, "mr-"+mrId)

		err := os.RemoveAll(path)
		if err != nil {
			log.Printf("Error while ripping env: %s - %s\n", "mr-"+mrId, err)
		} else {
			log.Printf("Reap outdated env: %s\n", "mr-"+mrId)
		}
		utils.ReplaceInFile(filepath.Join(base, "kustomization.yaml"), "\n  - ./mr-"+mrId, "")
	}

	ressourcesText := "resources:\n  -"
	if !utils.FileContains(filepath.Join(base, "kustomization.yaml"), ressourcesText) {
		utils.ReplaceInFile(filepath.Join(base, "kustomization.yaml"), "ressources:\n", "")
	}

	app.repo.AddAll()
	app.repo.Commit("[MR Controller] reaped old envs")
	app.repo.Push()
}

func (app *App) retrieveEnvironementStatus(mrId int) MrDeployStatus {
	commits, _, err := app.gitlab.MergeRequests.GetMergeRequestCommits(app.config.pid, mrId, &gitlab.GetMergeRequestCommitsOptions{PerPage: 1})
	// Latest commit
	if err != nil || len(commits) != 1 {
		log.Printf("No commit for MR %d - %s", mrId, err)
		return NotDeployed
	}
	latestCommit := commits[0]

	// TODO: Check if an image has been built ???

	app.repo.Pull()

	base := filepath.Join(app.repo.Folder, "apps/esap/mr")
	cloned := filepath.Join(base, "mr-"+strconv.Itoa(mrId))

	if _, err := os.Stat(cloned); os.IsNotExist(err) {
		return NotDeployed
	}

	// Check if the latest image tag is deployed
	valueFile := filepath.Join(cloned, "esap-values.yaml")
	b, err := os.ReadFile(valueFile)
	if err != nil {
		panic(err)
	}
	s := string(b)
	versionId := strconv.Itoa(int(latestCommit.CommittedDate.Unix()))
	if !strings.Contains(s, "tag: "+versionId) {
		return UpdateAvailable
	} else {
		return UpToDate
	}
}

func (app *App) updateMrMessageStatus(newMergeRequests []*gitlab.MergeRequest) {
	for _, mergeRequest := range newMergeRequests {

		deployementStatus := app.retrieveEnvironementStatus(mergeRequest.IID)

		messages, _, err := app.gitlab.Notes.ListMergeRequestNotes(app.config.pid, mergeRequest.IID, &gitlab.ListMergeRequestNotesOptions{})
		var botMessage *gitlab.Note = nil
		for _, message := range messages {
			if strings.HasPrefix(message.Body, "****") {
				botMessage = message
			}
		}
		if err != nil {
			log.Printf("Error while retrieving MR[%d] notes: %s", mergeRequest.IID, err)
			return
		}

		message := ""
		switch deployementStatus {
		case UpToDate:
			message = "****\nYour Merge Request is UP-TO-DATE and should be accessible with the following URL:\n- https://esap-mr-" +
				strconv.Itoa(mergeRequest.IID) + ".cta.cscs.ch/sdc-portal/\n****\n\nYou might need to wait a few minutes for the service to be online."
		case UpdateAvailable:
			message = "****\nA newer deployement version of your code is currently in the process of being deployed, once completed, it will be available here:\n- https://esap-mr-" +
				strconv.Itoa(mergeRequest.IID) + ".cta.cscs.ch/sdc-portal/\n****\n\nYou might need to wait a few minutes for the service to be online."
		case Desynchronized:
			message = "****\nThe deployement environment is DESYNCHRONISED with your Merge Request, this may be cause by a failing image build pipeline:\n- https://esap-mr-" +
				strconv.Itoa(mergeRequest.IID) + ".cta.cscs.ch/sdc-portal/\n****\n\nYou might need to wait a few minutes for the service to be online."
		case Pending:
			message = "****\nA newer deployement version of your code is currently building, once completed, it will be available here:\n- https://esap-mr-" +
				strconv.Itoa(mergeRequest.IID) + ".cta.cscs.ch/sdc-portal/\n****\n\nYou might need to wait a few minutes for the service to be online."
		case NotDeployed:
			message = "****\nYour Merge Request is not deployed yet, please verify that the build pipeline has succeeded !\n****"
		}

		if botMessage == nil {
			// New message
			_, _, err := app.gitlab.Notes.CreateMergeRequestNote(app.config.pid, mergeRequest.IID, &gitlab.CreateMergeRequestNoteOptions{
				Body: &message,
			})
			if err != nil {
				log.Printf("Error while creating note for MR %d: %s", mergeRequest.IID, err)
			} else {
				log.Printf("Note for MR %d created: %s", mergeRequest.IID, UpToDate)
			}
		} else {
			_, _, err := app.gitlab.Notes.UpdateMergeRequestNote(app.config.pid, mergeRequest.IID, botMessage.ID, &gitlab.UpdateMergeRequestNoteOptions{
				Body: &message,
			})
			if err != nil {
				log.Printf("Error while updating note for MR %d: %s", mergeRequest.IID, err)
			} else {
				// log.Printf("Note for MR %d updated: %s", mergeRequest.IID, UpToDate)
				continue
			}
		}
	}
}

func (app *App) loop() {
	// 0. Load existing environements -> OK
	// 1. Get open MR -> OK
	// 2. Detect closed MR
	// 2a. Delete environement
	// 3. Detect opened MR
	// 3a. Generate Helm Charts
	// 4. Update edited MR
	// 4a. Update environement
	// 5. Messages on GitLab MR

	existingEnvIds, err := app.loadCurrentEnv()
	if err != nil {
		log.Fatalf("Unable to load current env from flux repository: %s", err)
	}
	log.Printf("Loaded %s envs : %v\n", strconv.Itoa(len(existingEnvIds)), existingEnvIds)

	openedState := "opened"
	openMergeRequests, _, err := app.gitlab.MergeRequests.ListProjectMergeRequests(app.config.pid, &gitlab.ListProjectMergeRequestsOptions{
		TargetBranch: &app.config.targetBranch,
		State:        &openedState,
	})
	if err != nil {
		log.Printf("Unable to list project MR: %s\n", err)
		return
	}

	// Identify new MR
	openMergeRequestIds := []int{}
	// newMergeRequestIds := []int{}
	newMergeRequests := []*gitlab.MergeRequest{}
	for _, mergeRequest := range openMergeRequests {
		openMergeRequestIds = append(openMergeRequestIds, mergeRequest.IID)
		if !slices.Contains(existingEnvIds, mergeRequest.IID) {
			newMergeRequests = append(newMergeRequests, mergeRequest)
			// newMergeRequestIds = append(newMergeRequestIds, mergeRequest.IID)
		}
	}
	log.Printf("Loaded %s open MR: %v\n", strconv.Itoa(len(openMergeRequests)), openMergeRequestIds)

	if len(newMergeRequests) > 0 {
		app.spawnNewEnv(newMergeRequests, app.config.envPrefix)
	}

	// Identify env to reap
	envIdsToDrop := []int{}
	// envIdsToUpdate := []int{}
	for _, id := range existingEnvIds {
		if !slices.Contains(openMergeRequestIds, id) {
			envIdsToDrop = append(envIdsToDrop, id)
		}
		//  else if !slices.Contains(newMergeRequestIds, id) {
		// 	envIdsToUpdate = append(envIdsToUpdate, id)
		// }
	}
	if len(envIdsToDrop) > 0 {
		app.reapOldEnv(envIdsToDrop, app.config.envPrefix)
	}

	// Not needed anymore -> automated using ImagePolicy
	// app.updateEnv(envIdsToUpdate)

	// Messages
	app.updateMrMessageStatus(openMergeRequests)
}

func (app *App) Run() {

	ticker := time.NewTicker(2 * time.Minute)
	quit := make(chan bool)
	go func() {
		app.loop()
		for {
			log.Println("Loop start")
			select {
			case <-ticker.C:
				app.loop()
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

	config := &AppConfig{
		pid:          os.Getenv("GITLAB_PROJECT_ID"),
		targetBranch: os.Getenv("TARGET_BRANCH"),
		envPrefix:    os.Getenv("ENV_PREFIX"),
	}
	app := NewApp(repo, git, config)
	app.Run()
}

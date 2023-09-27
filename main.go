package main

import (
	"bytes"
	"context"
	"fmt"
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
	"github.com/xanzy/go-gitlab"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func loadCurrentEnv(clientset *kubernetes.Clientset, envPrefix string)([]int, error){
	// Load existing namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	namespacesIds := []int{}
	for _, namespace := range namespaces.Items {
		if strings.HasPrefix(namespace.Name, envPrefix){
			id, err := strconv.Atoi(strings.TrimPrefix(namespace.Name, envPrefix))

			if err != nil {
				log.Printf("Invalid namespace id detected: %s\n", err)
			} else {
				namespacesIds = append(namespacesIds, id)
			}
		}
	}
	return namespacesIds, nil
}

func replaceInFile(file string, search string, replace string) {
	input, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	output := bytes.Replace(input, []byte(search), []byte(replace), -1)

	if err = os.WriteFile(file, output, 0666); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func spawnNewEnv(repo *git.Repository, newMergeRequests []*gitlab.MergeRequest, envPrefix string){
	repo.Pull()

	for _, mergeRequest := range newMergeRequests {
		// Namespace
		mrId := strconv.Itoa(mergeRequest.ID)
 
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
			for _, file := range files{
				replaceInFile(file, searchValue, replaceValue)
			}

			replaceInFile(filepath.Join(base, "kustomization.yaml"), "resources:", "resources:\n  - mr-"+mrId+"/kustomization.yaml")
			log.Printf("Create new env: %s\n", "mr-"+mrId)
		}
	}

	err := repo.AddAll()
	if err != nil{
		log.Fatalf("Add all error: %s", err)
	}
	err = repo.Commit("[MR Controller] spawn new envs")
	if err != nil{
		log.Fatalf("Commit error: %s", err)
	}
	err = repo.Push()
	if err != nil{
		log.Fatalf("Push error: %s", err)
	}
}

func reapOldEnv(repo *git.Repository, envIdsToDrop []int, envPrefix string){
	repo.Pull()

	for _, envId := range envIdsToDrop {
		// Namespace
		mrId := strconv.Itoa(envId)
		base := filepath.Join(repo.Folder, "apps/esap/mr")
		path := filepath.Join(base, "mr-"+mrId)

		err := os.RemoveAll(path)
		if err != nil{
			log.Printf("Error while ripping env: %s - %s\n", "mr-"+mrId, err)
		} else {
			log.Printf("Reap outdated env: %s\n", "mr-"+mrId)
		}
		replaceInFile(base+"kustomization.yaml", "  - mr-"+mrId+"/kustomization.yaml\n", "")
	}
	
	repo.AddAll()
	repo.Commit("[MR Controller] reaped old envs")
	repo.Push()
}

func loop(clientset *kubernetes.Clientset, gitlabApi *gitlab.Client, repo *git.Repository){
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

	existingEnvIds, err := loadCurrentEnv(clientset, envPrefix)
	if err != nil {
		// TODO: Manage error
		log.Printf("Unable to load existing envs: %s\n", err)
	}
	log.Printf("Loaded  %s envs\n", strconv.Itoa(len(existingEnvIds)))

	openedState := "opened"
	openMergeRequests, _, err := gitlabApi.MergeRequests.ListProjectMergeRequests(projectId, &gitlab.ListProjectMergeRequestsOptions{
		TargetBranch: &targetBranch,
		State: &openedState,
	})
	if err != nil {
		// TODO: Manage error
		log.Printf("Unable to list project MR: %s\n", err)
	}
	log.Printf("Loaded %s open MR\n", strconv.Itoa(len(openMergeRequests)))
	
	// Identify new MR
	var openMergeRequestIds []int
	newMergeRequests := []*gitlab.MergeRequest{}
	for _, mergeRequest := range openMergeRequests {
		openMergeRequestIds = append(openMergeRequestIds, mergeRequest.ID)
		if !slices.Contains(existingEnvIds, mergeRequest.ID){
			newMergeRequests = append(newMergeRequests, mergeRequest)
		}
	}
	spawnNewEnv(repo, newMergeRequests, envPrefix)
	
	// Identify env to reap
	envIdsToDrop := []int{}
	for _, id := range existingEnvIds {
		if !slices.Contains(openMergeRequestIds, id) {
			envIdsToDrop = append(envIdsToDrop, id)
		}
	}
	reapOldEnv(repo, envIdsToDrop, envPrefix)
	
	// TODO: Identify env top update
}

func main() {
	log.Println("Starting server")

	err := os.Mkdir("/home/app/.ssh", 0700)
	if err != nil{
		log.Printf("Error creating .ssh folder: %s\n", err)
	}

	fileConfig, _ := os.Create("/home/app/.ssh/config")
	fileConfig.Write([]byte("IdentityFile /home/app/.ssh/id_ecdsa\n"))
	
	fileKnownHosts, _ := os.Create("/home/app/.ssh/known_hosts")
	fileKnownHosts.Write([]byte(os.Getenv("FLUX_KNOWN_HOSTS")))

	fileEcdsa, _ := os.Create("/home/app/.ssh/id_ecdsa")
	fileEcdsa.Write([]byte(os.Getenv("FLUX_IDENTITY")))
	fileEcdsa.Chmod(0600)

	fileEcdsaPub, _ := os.Create("/home/app/.ssh/id_ecdsa-pub")
	fileEcdsaPub.Write([]byte(os.Getenv("FLUX_IDENTITY_PUB")))
	fileEcdsaPub.Chmod(0644)

	// Flux repository
	repository := os.Getenv("FLUX_REPOSITORY")
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	repo, err := git.NewGit(dir, repository)
	if err != nil{
		log.Printf("Error while initialising main flux config repository: %s", err)
		// os.Exit(-1)
		time.Sleep(30 * time.Second)
	}

	// Watched repository
	gitlabUrl := os.Getenv("GITLAB_URL")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	git, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(gitlabUrl))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ticker := time.NewTicker(2 * time.Minute)
	quit := make(chan bool)
	go func() {
		loop(clientset, git, repo)
		for {
			log.Println("Loop start")
		    select {
			case <- ticker.C:
				loop(clientset, git, repo)
			case <- quit:
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
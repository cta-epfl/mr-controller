package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"
	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
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
			id, err := strconv.Atoi(strings.TrimPrefix(namespace.Namespace, envPrefix))
			if err != nil {
				print("Invalid namespace id detected")
			} else {
				namespacesIds = append(namespacesIds, id)
			}
		}
	}
	return namespacesIds, nil
}

func spawnNewEnv(clientset *kubernetes.Clientset, newMergeRequests []*gitlab.MergeRequest, envPrefix string){
	for _, mergeRequest := range newMergeRequests {
		// Namespace
		namespace := envPrefix + strconv.Itoa(mergeRequest.ID)
		clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind: "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})

		// TODO: ESAP
	}
}

func reapOldEnv(clientset *kubernetes.Clientset, envIdsToDrop []int, envPrefix string){
	for _, envId := range envIdsToDrop {
		// Namespace
		namespace := envPrefix + strconv.Itoa(envId)
		clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace,*metav1.NewDeleteOptions(0))
	}
}

func loop(clientset *kubernetes.Clientset, git *gitlab.Client){
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
	}

	openedState := "opened"
	openMergeRequests, _, err := git.MergeRequests.ListProjectMergeRequests(projectId, &gitlab.ListProjectMergeRequestsOptions{
		TargetBranch: &targetBranch,
		State: &openedState,
	})
	if err == nil {
		// TODO: Manage nill
	}

	// Identify new MR
	var openMergeRequestIds []int
	newMergeRequests := []*gitlab.MergeRequest{}
	for _, mergeRequest := range openMergeRequests {
		openMergeRequestIds = append(openMergeRequestIds, mergeRequest.ID)
		if !slices.Contains(existingEnvIds, mergeRequest.ID){
			newMergeRequests = append(newMergeRequests, mergeRequest)
		}
	}
	spawnNewEnv(clientset, newMergeRequests, envPrefix)

	// Identify env to reap
	envIdsToDrop := []int{}
	for _, id := range existingEnvIds {
		if !slices.Contains(openMergeRequestIds, id) {
			envIdsToDrop = append(envIdsToDrop, id)
		}
	}
	reapOldEnv(clientset, envIdsToDrop, envPrefix)
	
	// TODO: Identify env top update
}

func main() {
	log.Println("Starting")

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

	gitlabUrl := os.Getenv("GITLAB_URL")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	git, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(gitlabUrl))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ticker := time.NewTicker(2 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
		   select {
			case <- ticker.C:
				loop(clientset, git)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	quit <- true

	log.Println("Shutting down")
	os.Exit(0)
}
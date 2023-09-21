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

func spawnNewEnv(newMergeRequests []*gitlab.MergeRequest){

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
	projectId := os.Getenv("PROJECT_ID")
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

	var merge_request_ids []int
	for _, mr := range openMergeRequests{
		merge_request_ids = append(merge_request_ids, mr.ID)
	}

	// Identify new MR
	newMergeRequests := []*gitlab.MergeRequest{}
	for _, mergeRequest := range openMergeRequests {
		if !slices.Contains(existingEnvIds, mergeRequest.ID){
			newMergeRequests = append(newMergeRequests, mergeRequest)
		}
	}
	spawnNewEnv(newMergeRequests)

	// Identify env to reap
}

func main() {
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

	GITLAB_TOKEN := os.Getenv("GITLAB_TOKEN")
	git, err := gitlab.NewClient(GITLAB_TOKEN)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
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
}
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/xanzy/go-gitlab"
)

func loop(git *gitlab.Client){
	targetBranch := "main"
	openedState := "opened"
	merge_requests, _, err := git.MergeRequests.ListProjectMergeRequests("porjectId", &gitlab.ListProjectMergeRequestsOptions{
		TargetBranch: &targetBranch,
		State: &openedState,
	})

	if err == nil {
		// TODO: Manage nill
	}

	// TODO Identify new merge_requests

}

func main() {
    fmt.Println("Hello, world.")

	// TODO: implement
	// 1. Get open MR
	// 2. Generate Helm Charts
	// 3. Loop
	// 4. Detect closed MR
	// 5. Detect opened MR
	// 6. Update edited MR
	// 7. Messages on GitLab MR

	gitlab_token := "gitlab_token"
	git, err := gitlab.NewClient(gitlab_token)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
		   select {
			case <- ticker.C:
				loop(git)
			case <- quit:
				ticker.Stop()
				return
			}
		}
	 }()
	 


}
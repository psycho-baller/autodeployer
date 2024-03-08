package gh

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v39/github"
)

// waits for the image build workflow to complete
func WaitForWorkflow() {
	fmt.Println("[3/5] Waiting for workflow completion...")
	workflowComplete := false
	for i := 0; i < Globals.WorkflowRetryLimit; i++ {
		workflows, _, err := Globals.Client.Actions.ListRepositoryWorkflowRuns(Globals.Ctx, Globals.Owner, Globals.Repo, nil)
		if err != nil {
			fmt.Printf("Error when fetching workflows: %s\n", err)
			os.Exit(1)
		}

		workflowStatus := workflows.WorkflowRuns[0].GetStatus()
		if workflowStatus == "completed" {
			fmt.Println("\nWorkflow has successfully completed!")
			workflowComplete = true
			break
		} else {
			fmt.Printf("\rWorkflow status: %s. Time elapsed: %ds", workflowStatus, (i+1)*Globals.WorkflowRetryWaitSeconds)
			time.Sleep(time.Duration(Globals.WorkflowRetryWaitSeconds) * time.Second)
		}
	}
	if !workflowComplete {
		fmt.Println("Error: Workflow failed to complete within time limit.")
		os.Exit(1)
	}
}

// bumps the image version in the deployment repository
func BumpDeployment(oldTag string, newTag string) string {
	fmt.Printf("[4/5] Bumping image version in %s...\n", Globals.DeploymentsRepo)

	// 0. Check if the deployment repo exists and get the default branch
	deploymentsRepoGithub, _, err := Globals.Client.Repositories.Get(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo)
	if err != nil {
		fmt.Printf("Failed to fetch %s: %s\n", Globals.DeploymentsRepo, err)
		os.Exit(1)
	}
	defaultBranch := *deploymentsRepoGithub.DefaultBranch

	// 1. Get commit hash of the default branch
	fmt.Println("- Reading commit hash of default branch...")
	ref, _, err := Globals.Client.Git.GetRef(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo, "refs/heads/"+defaultBranch)
	if err != nil {
		fmt.Println("Failed to fetch default branch:", err)
		os.Exit(1)
	}
	defaultBranchSHA := ref.Object.GetSHA()

	// 2. Create new branch in deployment repo
	futureTag := strings.Split(newTag, "-rc")[0]
	// TODO: add an option to enable/disable username in the branch name
	username, err := getUsername()
	if err != nil {
		fmt.Println("Failed to get username, will use '' as the username for the new deployment branch")
		username = ""
	}
	newBranchNameRef := fmt.Sprintf("refs/heads/%s-%s-bump-%s", username, Globals.Repo, futureTag)
	newBranch := &github.Reference{
		Ref:    &newBranchNameRef,
		Object: &github.GitObject{SHA: &defaultBranchSHA},
	}
	_, _, err = Globals.Client.Git.GetRef(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo, newBranchNameRef)
	if err != nil {
    // Branch does not exist, create it
    _, _, err = Globals.Client.Git.CreateRef(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo, newBranch)
    if err != nil {
			fmt.Printf("Failed to create new branch %s: %s\n", newBranchNameRef, err)
			os.Exit(1)
    }
	} else {
    fmt.Printf("Branch %s already exists\n", newBranchNameRef)
	}

	// 3. Get deployment YAML file from the repository
	options := &github.RepositoryContentGetOptions{Ref: newBranchNameRef}
	fileContent, _, _, err := Globals.Client.Repositories.GetContents(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo, Globals.DeploymentYAMLPath, options)
	if err != nil {
		fmt.Printf("Failed to get file contents: %s\n", err)
		os.Exit(1)
	}
	decodedContent, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		fmt.Println("Failed to decode file content:", err)
		os.Exit(1)
	}

	// 4. Replace old tag with new tag in the content
	contentStr := string(decodedContent)
	newContentStr := strings.Replace(contentStr, oldTag, newTag, -1)

	// 5. Push the updated content to the new branch
	data := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Image tag bumped to %s using autodeployer", newTag)),
		Content: []byte(newContentStr),
		SHA:     fileContent.SHA,
		Branch:  &newBranchNameRef,
	}
	_, _, err = Globals.Client.Repositories.UpdateFile(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo, Globals.DeploymentYAMLPath, data)
	if err != nil {
		fmt.Printf("Failed to update the file %s: %s\n", Globals.DeploymentYAMLPath, err)
		os.Exit(1)
	}
	fmt.Printf("Successfully bumped image version in %s!\n", Globals.DeploymentYAMLPath)

	return newBranchNameRef
}

// triggers a workflow on the specified branch in the repository
func TriggerWorkflow(branchNameRef string, workflowName string) {
	fmt.Printf("[5/5] Triggering '%s' workflow on branch %s...\n", workflowName, branchNameRef)

	// Prepare payload for workflow dispatch event
	eventPayload := github.CreateWorkflowDispatchEventRequest{
		Ref: branchNameRef,
	}

	// Trigger the workflow dispatch event
	_, err := Globals.Client.Actions.CreateWorkflowDispatchEventByFileName(Globals.Ctx, Globals.Owner, Globals.DeploymentsRepo, workflowName, eventPayload)
	if err != nil {
		fmt.Printf("Failed to trigger '%s' workflow: %s\n", workflowName, err)
		return
	}

	fmt.Println("Successfully triggered 'deploy' workflow!")
}

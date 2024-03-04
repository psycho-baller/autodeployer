package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v39/github"
	"gopkg.in/yaml.v2"
)

// getNewReleaseTag determines the new release tag
func getNewReleaseTag(ctx context.Context, client *github.Client) string {
	fmt.Println("\n[1/5] Determining new release tag...")
	opt := &github.ListOptions{Page: 1, PerPage: 1}
	releases, _, err := client.Repositories.ListReleases(ctx, owner, repo, opt)
	if err != nil {
		fmt.Printf("Error when fetching releases: %s\n", err)
		os.Exit(1)
	}

	rawTag := strings.Split(releases[0].GetTagName(), "-rc")
	oldVersion := strings.Split(rawTag[0], ".")
	if len(rawTag) == 1 {
		lastVersion, _ := strconv.Atoi(oldVersion[len(oldVersion)-1])
		oldVersion[len(oldVersion)-1] = strconv.Itoa(lastVersion + 1)
	}
	newTag := strings.Join(oldVersion, ".")
	if isPrerelease {
		if len(rawTag) == 1 {
			newTag += "-rc1"
		} else {
			lastRC, _ := strconv.Atoi(rawTag[1])
			newTag += "-rc" + strconv.Itoa(lastRC+1)
		}
	}
	fmt.Printf("New release tag: %s\n", newTag)
	return newTag
}

// createNewRelease creates a new release
func createNewRelease(ctx context.Context, client *github.Client, newTag string) {
	fmt.Println("[2/5] Creating new release...")
	release := &github.RepositoryRelease{
		TagName:         github.String(newTag),
		TargetCommitish: github.String(branch),
		Name:            github.String(newTag),
		Body:            github.String("Release created using autodeployer(tm)."),
		Draft:           github.Bool(false),
		Prerelease:      github.Bool(isPrerelease),
	}
	_, _, err := client.Repositories.CreateRelease(ctx, owner, repo, release)
	if err != nil {
		fmt.Printf("Failed to create release: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Release %s was successfully created.\nWaiting 10 seconds before checking the image build workflow...\n", newTag)
	time.Sleep(10 * time.Second)
}

// waitForWorkflow waits for the image build workflow to complete
func waitForWorkflow(ctx context.Context, client *github.Client) {
	fmt.Println("[3/5] Waiting for workflow completion...")
	workflowComplete := false
	for i := 0; i < workflowRetryLimit; i++ {
		workflows, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, nil)
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
			fmt.Printf("\rWorkflow status: %s. Time elapsed: %ds", workflowStatus, (i+1)*workflowRetryWaitSeconds)
			time.Sleep(time.Duration(workflowRetryWaitSeconds) * time.Second)
		}
	}
	if !workflowComplete {
		fmt.Println("Error: Workflow failed to complete within time limit.")
		os.Exit(1)
	}
}

// bumps the image version in the deployment repository
func bumpDeployment(ctx context.Context, client *github.Client, newTag string) string {
	fmt.Printf("[4/5] Bumping image version in %s...\n", deploymentsRepo)

	// 0. Check if the deployment repo exists and get the default branch
	deploymentsRepoGithub, _, err := client.Repositories.Get(ctx, owner, deploymentsRepo)
	if err != nil {
		fmt.Printf("Failed to fetch %s: %s\n", deploymentsRepo, err)
		os.Exit(1)
	}
	defaultBranch := *deploymentsRepoGithub.DefaultBranch

	// 1. Get commit hash of the default branch
	fmt.Println("- Reading commit hash of default branch...")
	ref, _, err := client.Git.GetRef(ctx, owner, deploymentsRepo, "heads/"+defaultBranch)
	if err != nil {
		fmt.Println("Failed to fetch master branch:", err)
		os.Exit(1)
	}
	defaultBranchSHA := ref.Object.GetSHA()

	// 2. Create new branch in deployment repo
	newBranchName := fmt.Sprintf("refs/heads/%s-bump-%s", repo, newTag)
	newBranch := &github.Reference{
		Ref:    &newBranchName,
		Object: &github.GitObject{SHA: &defaultBranchSHA},
	}
	_, _, err = client.Git.CreateRef(ctx, owner, deploymentsRepo, newBranch)
	if err != nil {
		fmt.Printf("Failed to create new branch %s: %s\n", newBranchName, err)
		os.Exit(1)
	}

	// 3. Get deployment YAML file from the repository
	deploymentYAMLPath := configPath
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, deploymentsRepo, deploymentYAMLPath, nil)
	if err != nil {
		fmt.Printf("Failed to get file contents: %s\n", err)
		os.Exit(1)
	}
	decodedContent, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		fmt.Println("Failed to decode file content:", err)
		os.Exit(1)
	}

	// 4. Generate new content with updated tag and push to new branch
	fmt.Printf("- Updating image tag in %s...\n", configPath)
	var yamlData map[string]interface{}
	err = yaml.Unmarshal(decodedContent, &yamlData)
	if err != nil {
		fmt.Println("Failed to unmarshal YAML content:", err)
		os.Exit(1)
	}
	yamlData["image"] = map[string]string{"repository": configImageURL, "tag": newTag}
	newFileContent, err := yaml.Marshal(yamlData)
	if err != nil {
		fmt.Println("Failed to marshal YAML content:", err)
		os.Exit(1)
	}

	newContentBase64 := base64.StdEncoding.EncodeToString(newFileContent)
	data := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Image tag bumped to %s using autodeployer(tm)", newTag)),
		Content: []byte(newContentBase64),
		SHA:     fileContent.SHA,
		Branch:  &newBranchName,
	}
	_, _, err = client.Repositories.UpdateFile(ctx, owner, deploymentsRepo, deploymentYAMLPath, data)
	if err != nil {
		fmt.Printf("Failed to update the file %s: %s\n", configPath, err)
		os.Exit(1)
	}
	fmt.Printf("Successfully bumped image version in %s!\n", configPath)

	return newBranchName
}

// triggerWorkflow triggers a workflow on the specified branch in the repository
func triggerWorkflow(ctx context.Context, client *github.Client, branchName string) {
	fmt.Printf("[5/5] Triggering 'deploy' workflow on branch %s...\n", branchName)

	// Prepare payload for workflow dispatch event
	// eventPayload := map[string]interface{}{
	// 	"ref": branchName,
	// 	"inputs": map[string]interface{}{
	// 		"workflow": "deploy",
	// 	},
	// }

	// Trigger the workflow dispatch event
	// _, _, err := client.Dispatches.CreateDispatchEvent(ctx, owner, repo, eventPayload)`
	// if err != nil {
	// 	fmt.Printf("Failed to trigger 'deploy' workflow: %s\n", err)
	// 	return
	// }

	fmt.Println("Successfully triggered 'deploy' workflow!")
}

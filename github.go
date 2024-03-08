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
)

type VersionChangeType string
const (
	Minor    VersionChangeType = "minor"
	Major    VersionChangeType = "major"
	Breaking VersionChangeType = "breaking"
)

func getLatestTagFromBranch(ctx context.Context, client *github.Client, owner string, repo string, branch string, tags []*github.RepositoryTag) (*github.RepositoryTag, error) {
	// Get the branch
	gitBranch, _, err := client.Repositories.GetBranch(ctx, owner, repo, branch, false)
	if err != nil {
			return nil, err
	}

	// Get the commit SHA from the branch
	commitSHA := *gitBranch.Commit.SHA

	// Find the tag that matches the commit SHA
	for _, tag := range tags {
			if *tag.Commit.SHA == commitSHA {
					return tag, nil
			}
	}
	// No tag found for the branch (which means it's their first deployment for the branch)
	return nil, nil
}

func getNewTag(oldTag string, versionChangeType VersionChangeType) (string, error) {
	if versionChangeType == Minor { // Minor and rc changes are the same (just increment the last number by 1)
		// get the last number of the tag and increment it by 1
		lastVersion := string(oldTag[len(oldTag)-1])
		minorVersion, err := strconv.Atoi(lastVersion)
		if err != nil {
				return "", fmt.Errorf("error when converting last version to int: %s", err)
		}
		minorVersion++
		newTag := fmt.Sprintf("%s%d", oldTag[:len(oldTag)-1], minorVersion)
		isFirstRC := !strings.Contains(oldTag, "-rc")
		if isFirstRC {
			newTag += "-rc1"
		}
		return newTag, nil
	}

	return "", fmt.Errorf("major and breaking changes have not been implemented") // Add a return statement with an error message to fix the "missing return" problem
}

// getNewReleaseTag determines the new release tag
func getOldAndNewReleaseTag(ctx context.Context, client *github.Client, versionChageType VersionChangeType) (string, string, error) {
	// set default params if not provided
	if versionChageType == "" {
		versionChageType = Minor
	}
	fmt.Println("\n[1/5] Determining new release tag...")
	opt := &github.ListOptions{Page: 1, PerPage: 1}
	
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, opt)
	if err != nil {
		fmt.Printf("Error when fetching releases: %s\n", err)
		os.Exit(1)
	}
	latestTagFromBranch, err := getLatestTagFromBranch(ctx, client, owner, repo, branch, tags)
	if err != nil {
		fmt.Printf("Error when fetching latest tag: %s\n", err)
		os.Exit(1)
	}
	oldTag := tags[0].GetName()
	isFirstRC := latestTagFromBranch == nil
	if isFirstRC {
		fmt.Println("No tags found for the branch. Assuming this is the first deployment attempt.")
		oldTag = latestTagFromBranch.GetName()
	}
	newTag, err := getNewTag(oldTag, versionChageType)
	if err != nil {
			return oldTag, "", err
	}

	return oldTag, newTag, nil
}
	// rawTag := strings.Split(releases[0].GetTagName(), "-rc")
	// oldTag := rawTag[0]
	// oldVersion := strings.Split(oldTag, ".")
	// if len(rawTag) == 1 {
	// 	lastVersion, _ := strconv.Atoi(oldVersion[len(oldVersion)-1])
	// 	oldVersion[len(oldVersion)-1] = strconv.Itoa(lastVersion + 1)
	// }
	// newTag := strings.Join(oldVersion, ".")
	// if isPrerelease {
	// 	if len(rawTag) == 1 {
	// 		newTag += "-rc1"
	// 	} else {
	// 		lastRC, _ := strconv.Atoi(rawTag[1])
	// 		newTag += "-rc" + strconv.Itoa(lastRC+1)
	// 	}
	// }
	// return oldTag, newTag

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
	fmt.Printf("Release %s was successfully created.\n", newTag)
	// fmt.Println("Waiting 10 seconds before checking the image build workflow...\n")
	// time.Sleep(10 * time.Second)
}

// waits for the image build workflow to complete
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
func bumpDeployment(ctx context.Context, client *github.Client, oldTag string, newTag string) string {
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
	ref, _, err := client.Git.GetRef(ctx, owner, deploymentsRepo, "refs/heads/"+defaultBranch)
	if err != nil {
		fmt.Println("Failed to fetch master branch:", err)
		os.Exit(1)
	}
	defaultBranchSHA := ref.Object.GetSHA()

	// 2. Create new branch in deployment repo
	newBranchNameRef := fmt.Sprintf("refs/heads/%s-bump", repo)
	newBranch := &github.Reference{
		Ref:    &newBranchNameRef,
		Object: &github.GitObject{SHA: &defaultBranchSHA},
	}
	_, _, err = client.Git.GetRef(ctx, owner, deploymentsRepo, newBranchNameRef)
	if err != nil {
    // Branch does not exist, create it
    _, _, err = client.Git.CreateRef(ctx, owner, deploymentsRepo, newBranch)
    if err != nil {
			fmt.Printf("Failed to create new branch %s: %s\n", newBranchNameRef, err)
			os.Exit(1)
    }
	} else {
    fmt.Printf("Branch %s already exists\n", newBranchNameRef)
	}

	// 3. Get deployment YAML file from the repository
	deploymentYAMLPath := configPath
	options := &github.RepositoryContentGetOptions{Ref: newBranchNameRef}
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, deploymentsRepo, deploymentYAMLPath, options)
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
	_, _, err = client.Repositories.UpdateFile(ctx, owner, deploymentsRepo, deploymentYAMLPath, data)
	if err != nil {
		fmt.Printf("Failed to update the file %s: %s\n", deploymentYAMLPath, err)
		os.Exit(1)
	}
	fmt.Printf("Successfully bumped image version in %s!\n", deploymentYAMLPath)

	return newBranchNameRef
}

// triggers a workflow on the specified branch in the repository
func triggerWorkflow(ctx context.Context, client *github.Client, branchNameRef string, workflowName string) {
	fmt.Printf("[5/5] Triggering '%s' workflow on branch %s...\n", workflowName, branchNameRef)

	// Prepare payload for workflow dispatch event
	eventPayload := github.CreateWorkflowDispatchEventRequest{
		Ref: branchNameRef,
	}

	// Trigger the workflow dispatch event
	_, err := client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, deploymentsRepo, workflowName, eventPayload)
	if err != nil {
		fmt.Printf("Failed to trigger '%s' workflow: %s\n", workflowName, err)
		return
	}

	fmt.Println("Successfully triggered 'deploy' workflow!")
}

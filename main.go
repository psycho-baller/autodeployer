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
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

// Configuration struct for holding settings from config.yaml
type Configuration struct {
	Settings        map[string]string                       `yaml:"settings"`
	DeploymentRepos map[string]map[string]map[string]string `yaml:"deployment_repos"`
}

// Auth struct for holding token from auth.yaml
type Auth struct {
	Auth struct {
		Token string `yaml:"token"`
	} `yaml:"auth"`
}

var (
	owner                    string
	workflowRetryLimit       int
	workflowRetryWaitSeconds int
	deploymentsRepo          string
	configPath               string
	configImageURL           string
	isPrerelease             bool = true
	repo                     string
	branch                   string
)

func main() {
	// Parse arguments
	if len(os.Args) <= 2 {
		fmt.Println("Usage: go run autodeployer.go <REPO_NAME> <BRANCH_NAME>")
		os.Exit(1)
	}
	repo = os.Args[1]
	branch = os.Args[2]

	// Parse auth.yaml
	authData, err := os.ReadFile("auth.yaml")
	if err != nil {
		fmt.Println("Error reading auth.yaml:", err)
		os.Exit(1)
	}
	var auth Auth
	err = yaml.Unmarshal(authData, &auth)
	if err != nil {
		fmt.Println("Error parsing auth.yaml:", err)
		os.Exit(1)
	}
	token := auth.Auth.Token

	// Parse config.yaml
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		fmt.Println("Error reading config.yaml:", err)
		os.Exit(1)
	}
	var config Configuration
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		fmt.Println("Error parsing config.yaml:", err)
		os.Exit(1)
	}
	owner = config.Settings["owner"]
	workflowRetryLimit, _ = strconv.Atoi(config.Settings["workflow_retry_limit"])
	workflowRetryWaitSeconds, _ = strconv.Atoi(config.Settings["workflow_retry_wait_seconds"])
	deploymentsRepo = getDeploymentRepo(repo, config.DeploymentRepos)
	if deploymentsRepo == "" {
		fmt.Printf("Deployment repo not found for %s\n", repo)
		os.Exit(1)
	}
	configPath = config.DeploymentRepos[deploymentsRepo][repo]["staging-config-path"]
	configImageURL = config.DeploymentRepos[deploymentsRepo][repo]["config-image-url"]

	// Create GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Get new release tag
	newTag := getNewReleaseTag(ctx, client)
	fmt.Println(newTag)
	// createNewRelease(ctx, client, newTag)
	// waitForWorkflow(ctx, client)
	// bumpDeployment(ctx, client, newTag)

	// Send notification
	fmt.Println("Deployment Successful! Autodeployer terminating...")
}

// getDeploymentRepo retrieves the deployment repo from `repoName` argument
func getDeploymentRepo(repoName string, deploymentRepos map[string]map[string]map[string]string) string {
	for deploymentRepo, repos := range deploymentRepos {
		for repo := range repos {
			if repo == repoName {
				fmt.Printf("Found deployment repo: %s\n", deploymentRepo)
				return deploymentRepo
			}
		}
	}
	return ""
}

// getNewReleaseTag determines the new release tag
func getNewReleaseTag(ctx context.Context, client *github.Client) string {
	fmt.Println("\n[1/4] Determining new release tag...")
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
	fmt.Println("[2/4] Creating new release...")
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
	fmt.Println("[3/4] Waiting for workflow completion...")
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

// bumpDeployment bumps the image version in the deployment repository
func bumpDeployment(ctx context.Context, client *github.Client, newTag string) {
	fmt.Printf("[4/4] Bumping image version in %s...\n", deploymentsRepo)

	// 1. Get commit hash of master branch
	fmt.Println("- Reading commit hash of master branch...")
	ref, _, err := client.Git.GetRef(ctx, owner, deploymentsRepo, "heads/master")
	if err != nil {
		fmt.Println("Failed to fetch master branch:", err)
		os.Exit(1)
	}
	masterBranchSHA := ref.Object.GetSHA()

	// 2. Create new branch in deployment repo
	newBranchName := fmt.Sprintf("refs/heads/%s-bump-%s", repo, newTag)
	newBranch := &github.Reference{
		Ref:    &newBranchName,
		Object: &github.GitObject{SHA: &masterBranchSHA},
	}
	_, _, err = client.Git.CreateRef(ctx, owner, deploymentsRepo, newBranch)
	if err != nil {
		fmt.Printf("Failed to create a new branch in %s: %s\n", deploymentsRepo, err)
		os.Exit(1)
	}

	// 3. Fetch contents of deployment yaml file
	deploymentYAMLPath := fmt.Sprintf("contents/%s", configPath)
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
}

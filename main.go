package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"

	gh "github.com/psycho-baller/autodeployer/github"
)

// Configuration struct for holding settings from config.yaml
type Configuration struct {
	Settings        map[string]string                       `yaml:"settings"`
	DeploymentRepos map[string]map[string]map[string]string `yaml:"deployment_repos"`
}

var (
	owner                    string
	workflowRetryLimit       int
	workflowRetryWaitSeconds int
	deploymentsRepo          string
	deploymentYAMLPath       string
	configImageURL           string
	isPrerelease             bool = true
	repo                     string
	branch                   string
)

func main() {
	// Parse arguments
	if len(os.Args) <= 2 {
		fmt.Println("Usage: go run github.com/psycho-baller/autodeployer <REPO_NAME> <BRANCH_NAME>")
		os.Exit(1)
	}
	
	token := getGHECToken()
	
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
	repo = os.Args[1]
	branch = os.Args[2]
	owner = config.Settings["owner"]
	workflowRetryLimit, _ = strconv.Atoi(config.Settings["workflow_retry_limit"])
	workflowRetryWaitSeconds, _ = strconv.Atoi(config.Settings["workflow_retry_wait_seconds"])
	deploymentsRepo = GetDeploymentRepo(repo, config.DeploymentRepos)
	if deploymentsRepo == "" {
		fmt.Printf("Deployment repo not found for %s\n", repo)
		os.Exit(1)
	}
	deploymentYAMLPath = config.DeploymentRepos[deploymentsRepo][repo]["staging-config-path"]
	configImageURL = config.DeploymentRepos[deploymentsRepo][repo]["config-image-url"]
	// Create GitHub client
	ghCtx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ghCtx, ts)
	client := github.NewClient(tc)

	gh.Globals = gh.AppContext{
    Owner:                    owner,
    Repo:                     repo,
    Branch:                   branch,
    DeploymentsRepo:          deploymentsRepo,
    DeploymentYAMLPath:       deploymentYAMLPath,
    WorkflowRetryLimit:       workflowRetryLimit,
    WorkflowRetryWaitSeconds: workflowRetryWaitSeconds,
    ConfigImageURL:           configImageURL,
    IsPrerelease:             isPrerelease,
    Ctx:                      ghCtx,
    Client:                   client,
}

	// Get new release tag
	oldTag, newTag, err := gh.GetOldAndNewReleaseTag("")
	if err != nil {
		fmt.Println("Error getting old and new release tag:", err)
		os.Exit(1)
	}
	fmt.Println("Old release tag:", oldTag)
	fmt.Println("New release tag:", newTag)
	gh.CreateNewRelease(newTag)
	newBranchRef := gh.BumpDeployment(oldTag, newTag)
	// TODO: Add option to skip this step
	fmt.Printf("[4/5] Triggering '%s' workflow on branch %s...\n", "deploy.yaml", newBranchRef)
	gh.TriggerWorkflow(newBranchRef, "deploy.yaml")
	// 3. Wait for the image build workflow to complete
	// TODO: Add option to skip this step
	announce(Notification, fmt.Sprintf("Deploying to %s", deploymentsRepo), fmt.Sprintf("Successfully triggered deployment workflow for %s in %s through %s", newTag, repo, deploymentsRepo))
	// Waiting 5 seconds before checking the image build workflow...
	time.Sleep(5 * time.Second)
	fmt.Println("[5/5] Waiting for workflow completion...")
	gh.WaitForWorkflow(deploymentsRepo, strings.Split(newBranchRef, "heads/")[1])
	announce(Alert,fmt.Sprintf("%s branch in %s has been deployed through %s", branch, repo, deploymentsRepo),fmt.Sprintf("Old release tag: %s\nNew release tag: %s", oldTag, newTag))
	fmt.Println("Deployment Successful! Autodeployer terminating...")
}

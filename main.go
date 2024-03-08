package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
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
	owner = config.Settings["owner"]
	workflowRetryLimit, _ = strconv.Atoi(config.Settings["workflow_retry_limit"])
	workflowRetryWaitSeconds, _ = strconv.Atoi(config.Settings["workflow_retry_wait_seconds"])
	deploymentsRepo = GetDeploymentRepo(repo, config.DeploymentRepos)
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
	oldTag, newTag, err := getOldAndNewReleaseTag(ctx, client, "")
	if err != nil {
		fmt.Println("Error getting old and new release tag:", err)
		os.Exit(1)
	}
	fmt.Println("Old release tag:", oldTag)
	fmt.Println("New release tag:", newTag)
	createNewRelease(ctx, client, newTag)
	// waitForWorkflow(ctx, client)
	newBranchRef := bumpDeployment(ctx, client, oldTag, newTag)
	triggerWorkflow(ctx, client, newBranchRef, "deploy.yaml")
	// Send notification
	fmt.Println("Deployment Successful! Autodeployer terminating...")
}

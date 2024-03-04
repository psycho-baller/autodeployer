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
	newTag := "1.7.5-rc1"//getNewReleaseTag(ctx, client)
	// fmt.Println("New release tag:", newTag)
	// createNewRelease(ctx, client, newTag)
	// waitForWorkflow(ctx, client)
	newBranch := bumpDeployment(ctx, client, newTag)
	fmt.Println("New branch:", newBranch)
	triggerWorkflow(ctx, client, newBranch)
	// Send notification
	fmt.Println("Deployment Successful! Autodeployer terminating...")
}

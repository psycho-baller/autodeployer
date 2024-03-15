package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v39/github"
	gh "github.com/psycho-baller/autodeployer/github"
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
	deploymentYAMLPath       string
	configImageURL           string
	isPrerelease             bool = true
	repo                     string
	branch                   string
	userDefinedOldTag        string
)

func main() {
	// Parse arguments
	if len(os.Args) <= 2 {
		// check if they have an environment variable set
		if os.Getenv("AD_REPO") != "" && os.Getenv("AD_BRANCH") != "" {
			repo = os.Getenv("AD_REPO")
			branch = os.Getenv("AD_BRANCH")
		} else {
			fmt.Println("Usage: go run github.com/psycho-baller/autodeployer <REPO_NAME> <BRANCH_NAME>")
			os.Exit(1)
		}
	} else {
		repo = os.Args[1]
		branch = os.Args[2]
	}
	if len(os.Args) > 3 {
		userDefinedOldTag = os.Args[3]
	}

	token := getGHECToken()

	// Parse config.yaml
	wd, err := os.Getwd()
    if err != nil {
        panic(err)
    }
	fmt.Println(wd)
    var configPath string
	if strings.Contains(wd, "bin") {
		// If you're running the binary from the bin directory
		configPath = filepath.Join(filepath.Dir(wd), "config.yaml")
	} else {
		// If you're running the go file directly
		configPath = filepath.Join(wd, "config.yaml")
	}

    configData, err := os.ReadFile(configPath)
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
	UserDefinedOldTag:        userDefinedOldTag,
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
	fmt.Println("Waiting for image build workflow to complete...")
	gh.WaitForWorkflow(repo, branch)
	newBranchRef := gh.BumpDeployment(oldTag, newTag)
	// TODO: Add option to skip this step
	deploymentYAMLPath = config.DeploymentRepos[deploymentsRepo][repo]["staging-config-path"]
	var workflowName string
	if deploymentsRepo == "apps-faculty-deploy" {
		workflowName = "deploy.yml"
	} else {
		workflowName = "deploy.yaml"
	}
	fmt.Printf("[4/5] Triggering '%s' workflow on branch %s...\n", workflowName, newBranchRef)
	gh.TriggerWorkflow(newBranchRef, workflowName)
	// 3. Wait for the image build workflow to complete
	// TODO: Add option to skip this step
	announce(Notification, fmt.Sprintf("Deploying to %s", deploymentsRepo), fmt.Sprintf("Successfully triggered deployment workflow for %s in %s through %s", newTag, repo, deploymentsRepo))
	// Waiting 5 seconds before checking the image build workflow...
	time.Sleep(5 * time.Second)
	fmt.Println("[5/5] Waiting for deployment workflow to complete...")
	gh.WaitForWorkflow(deploymentsRepo, strings.Split(newBranchRef, "heads/")[1])
	announce(Alert,fmt.Sprintf("%s branch in %s has been deployed through %s", branch, repo, deploymentsRepo),fmt.Sprintf("Old release tag: %s\nNew release tag: %s", oldTag, newTag))
	fmt.Println("Deployment Successful! Autodeployer terminating...")
}

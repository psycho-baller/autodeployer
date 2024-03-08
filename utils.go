package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GetDeploymentRepo retrieves the deployment repo from `repoName` argument
func GetDeploymentRepo(repoName string, deploymentRepos map[string]map[string]map[string]string) string {
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

func getGHECToken() string {
	cmd := exec.Command("op", "read", "op://Private/GHEC_TOKEN/token")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error fetching GHEC token from 1Password:", err)
		os.Exit(1)
	}
	token := strings.TrimSpace(string(output))
	return token
}
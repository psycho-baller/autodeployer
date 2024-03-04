package main

import "fmt"

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

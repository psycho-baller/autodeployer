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

type NotificationType string

const (
    Alert        NotificationType = "alert"
    Notification NotificationType = "notification"
		Dialog       NotificationType = "dialog"
)
func announce(notificationType NotificationType, title, message string) {
	var script string
	switch notificationType {
		case Alert:
			script = fmt.Sprintf(`display alert "%s" message "%s" as informational giving up after 15`, title, message)
		case Notification:
			script = fmt.Sprintf(`tell app "System Events" to display notification "%s" with title "%s"`, message, title)
		case Dialog:
			script = `display dialog "Hello" with icon success buttons {"OK"} default button "OK"`
		default:
			fmt.Println("Invalid notification type:", notificationType)
			return
	}
	cmd := exec.Command("osascript", "-e", script)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error displaying notification:", err)
	}
}
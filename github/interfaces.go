package gh

import (
	"context"

	"github.com/google/go-github/v39/github"
)

type AppContext struct {
	Owner                    string
	Repo                     string
	Branch                   string
	DeploymentsRepo          string
	DeploymentYAMLPath			 string
	WorkflowRetryLimit       int
	WorkflowRetryWaitSeconds int
	ConfigImageURL           string
	IsPrerelease             bool
	Ctx                      context.Context
	Client                   *github.Client
}

var Globals AppContext

type VersionChangeType string
const (
	Minor    VersionChangeType = "minor"
	Major    VersionChangeType = "major"
	Breaking VersionChangeType = "breaking"
)

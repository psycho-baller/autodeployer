package gh

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v39/github"
)
func getLatestTagFromBranch(branch string, tags []*github.RepositoryTag) (*github.RepositoryTag, error) {
	// Get the branch
	gitBranch, _, err := Globals.Client.Repositories.GetBranch(Globals.Ctx, Globals.Owner, Globals.Repo, branch, false)
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


func getMostRecentTags(options *github.ListOptions) []*github.RepositoryTag {
	tags, _, err := Globals.Client.Repositories.ListTags(Globals.Ctx, Globals.Owner, Globals.Repo, options)
	if err != nil {
		fmt.Printf("Error when fetching tags: %s\n", err)
		os.Exit(1)
	}
	return tags
}


// getNewReleaseTag determines the new release tag
func GetOldAndNewReleaseTag(versionChageType VersionChangeType) (string, string, error) {
	// set default params if not provided
	if versionChageType == "" {
		versionChageType = Minor
	}
	fmt.Println("\n[1/5] Determining new release tag...")
	options := &github.ListOptions{PerPage: 10}
	tags := getMostRecentTags(options)
	latestTagFromBranch, err := getLatestTagFromBranch(Globals.Branch, tags)
	if err != nil {
		fmt.Printf("Error when fetching latest tag: %s\n", err)
		os.Exit(1)
	}
	for _, tag := range tags {
		fmt.Printf("Tag: %s\n", tag.GetName())
	}
	oldTag := tags[0].GetName()
	isFirstRC := latestTagFromBranch == nil
	if isFirstRC {
		fmt.Println("No tags found for the branch. Assuming this is the first deployment attempt.")
		// latestTag := tags[0].GetName()
		// oldTag = 
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
func CreateNewRelease(newTag string) {
	fmt.Println("[2/5] Creating new release...")
	release := &github.RepositoryRelease{
		TagName:         github.String(newTag),
		TargetCommitish: github.String(Globals.Branch),
		Name:            github.String(newTag),
		Body:            github.String("Release created using autodeployer"),
		Draft:           github.Bool(false),
		Prerelease:      github.Bool(Globals.IsPrerelease),
	}
	_, _, err := Globals.Client.Repositories.CreateRelease(Globals.Ctx, Globals.Owner, Globals.Repo, release)
	if err != nil {
		fmt.Printf("Failed to create release: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Release %s was successfully created.\n", newTag)
	// fmt.Println("Waiting 10 seconds before checking the image build workflow...\n")
	// time.Sleep(10 * time.Second)
}
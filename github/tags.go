package gh

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v39/github"
)

func printTags(tags []*github.RepositoryTag) {
	for _, tag := range tags {
		fmt.Printf("Tag: %s\n", tag.GetName())
	}
}

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

func filterTagsByUser(tags []*github.RepositoryTag, user string) ([]*github.RepositoryTag, error) {
	var filteredTags []*github.RepositoryTag
	for _, tag := range tags {
		commit, _, err := Globals.Client.Repositories.GetCommit(Globals.Ctx, Globals.Owner, Globals.Repo, *tag.Commit.SHA, nil)
		if err != nil {
			return nil, err
		}

		if commit.Author != nil && commit.Author.GetLogin() == user {
			filteredTags = append(filteredTags, tag)
		}
	}
	return filteredTags, nil
}

// doesn't work yet
func filterTagsbyDayCutoff(tags []*github.RepositoryTag, daysCutoff int) ([]*github.RepositoryTag, error) {
	var tagList []*github.RepositoryTag
	cutoff := time.Now().AddDate(0, 0, -daysCutoff)
	for _, tag := range tags {
		// the issue ⬇️
		date := tag.GetCommit().GetCommitter().GetDate().Format(time.RFC3339)
		fmt.Printf("Date: %s\n", date)
		parsedDate, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return nil, fmt.Errorf("error parsing date: %w", err)
		}
		if parsedDate.Before(cutoff) {
			continue
		}
		tagList = append(tagList, tag)
	}

	sort.Slice(tagList, func(i, j int) bool {
		return tagList[i].GetCommit().GetCommitter().GetDate().Before(tagList[j].GetCommit().GetCommitter().GetDate())
	})

	return tagList, nil
}

func getLatestOfficialReleaseTag(repo string) (*github.RepositoryRelease, error) {
	releases, _, err := Globals.Client.Repositories.ListReleases(Globals.Ctx, Globals.Owner, repo, nil)
	if err != nil {
			return nil, fmt.Errorf("error fetching releases: %w", err)
	}
	for _, release := range releases {
		fmt.Printf("Release: %s\n", release.GetTagName())
			// if release.GetPrerelease() {
			// 		continue
			// }
			return release, nil
	}

	return nil, fmt.Errorf("no non-pre-release found for repository %s/%s", Globals.Owner, repo)
}

// getNewReleaseTag determines the new release tag
func GetOldAndNewReleaseTag(versionChageType VersionChangeType) (string, string, error) {
	// 0. set default params if not provided
	if versionChageType == "" {
		versionChageType = Minor
	}
	
	// 1. get the tags and apply filters to them for more accurate results
	fmt.Println("\n[1/5] Determining new release tag...")
	options := &github.ListOptions{}
	tags := getMostRecentTags(options)
	// filter tags by the last 30 days
	// finteredTagsByDayCutoff, err := filterTagsbyDayCutoff(tags, 30)
	// if err != nil {
	// 	fmt.Printf("Error when filtering tags by day cutoff: %s\nWill not filter out tags older than 30 days.\n", err)
	// } else {
	// 	fmt.Printf("Filtering tags by day cutoff: %t\n", len(tags) == len(finteredTagsByDayCutoff))
	// 	tags = finteredTagsByDayCutoff
	// }

	// filter even more based on the users who created the tag
	username, err := getUsername()
	if err != nil {
		fmt.Printf("Error when fetching username: %s\nWill not filter out tags not created by you.\n", err)
	} else {
		filteredTagsByUser, err := filterTagsByUser(tags, username)
		if err != nil {
			fmt.Printf("Error when filtering tags by user: %s\nWill not filter out tags not created by you.\n", err)
		} else {
			fmt.Printf("Filtering tags by user: %t\n", len(tags) == len(filteredTagsByUser))

			tags = filteredTagsByUser
		}
	}

	// 2. get the latest tag from the branch
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
		latestOfficialReleaseTag, err := getLatestOfficialReleaseTag(Globals.Repo)
		if err != nil {
			fmt.Printf("Error when fetching latest official release tag: %s\n", err)
			os.Exit(1)
		}
		oldTag = latestOfficialReleaseTag.GetTagName()
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
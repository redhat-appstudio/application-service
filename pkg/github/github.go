package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/go-github/v41/github"
	"github.com/redhat-appstudio/application-service/pkg/util"
)

const AppStudioAppDataOrg = "redhat-appstudio-appdata"
const AppStudioAppDataURL = "https://github.com/" + AppStudioAppDataOrg + "/"

func GenerateNewRepositoryName(displayName string, namespace string) string {
	sanitizedName := util.SanitizeDisplayName(displayName)

	repoName := sanitizedName + "-" + namespace + "-" + gofakeit.Verb() + "-" + gofakeit.Noun()
	return repoName
}

func GenerateNewRepository(client *github.Client, ctx context.Context, repoName string, description string) (string, error) {
	isPrivate := true

	r := &github.Repository{Name: &repoName, Private: &isPrivate, Description: &description}
	_, _, err := client.Repositories.Create(ctx, AppStudioAppDataOrg, r)
	if err != nil {
		return "", err
	}
	repoURL := AppStudioAppDataURL + repoName
	return repoURL, nil
}

// GetRepoNameFromURL returns the repository name from the Git repo URL
func GetRepoNameFromURL(repoURL string, orgName string) (string, error) {
	parts := strings.Split(repoURL, orgName+"/")
	if len(parts) < 2 {
		return "", fmt.Errorf("error: unable to parse Git repository URL: %v", repoURL)
	}
	fmt.Println(parts)
	return parts[1], nil
}

// Delete Repository takes in the given repository URL and attempts to delete it
func DeleteRepository(client *github.Client, ctx context.Context, orgName string, repoName string) error {
	// Retrieve just the repository name from the URL
	_, err := client.Repositories.Delete(ctx, orgName, repoName)
	if err != nil {
		return err
	}
	return nil
}

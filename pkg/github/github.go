//
// Copyright 2021-2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/go-github/v41/github"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/redhat-appstudio/application-service/pkg/util"
)

const AppStudioAppDataOrg = "redhat-appstudio-appdata"

// ServerError is used to identify gitops repo creation failures caused by server errors
type ServerError struct {
	err error
}

func (e *ServerError) Error() string {
	return fmt.Errorf("failed to create gitops repo due to error: %v", e.err).Error()
}

// GenerateNewRepositoryName creates a new gitops repository name, based on the following format:
// <display-name>-<partial-hash-of-clustername-and-namespace>-<random-word>-<random-word>
func GenerateNewRepositoryName(displayName, uniqueHash string) string {
	sanitizedName := util.SanitizeName(displayName)
	repoName := sanitizedName + "-" + uniqueHash + "-" + util.SanitizeName(gofakeit.Verb()) + "-" + util.SanitizeName(gofakeit.Verb())
	return repoName
}

func GenerateNewRepository(client *github.Client, ctx context.Context, orgName string, repoName string, description string) (string, error) {
	isPrivate := false
	appStudioAppDataURL := "https://github.com/" + orgName + "/"
	metrics.GitOpsRepoCreationTotalReqs.Inc()
	r := &github.Repository{Name: &repoName, Private: &isPrivate, Description: &description}
	_, resp, err := client.Repositories.Create(ctx, orgName, r)

	if 500 <= resp.StatusCode && resp.StatusCode <= 599 {
		// return custom error
		if err != nil {
			metrics.GitOpsRepoCreationFailed.Inc()
			return "", &ServerError{err: err}
		}
	}

	if err != nil {
		return "", err
	}
	repoURL := appStudioAppDataURL + repoName
	metrics.GitOpsRepoCreationSucceeded.Inc()
	return repoURL, nil
}

// GetRepoNameFromURL returns the repository name from the Git repo URL
func GetRepoNameFromURL(repoURL string, orgName string) (string, error) {
	parts := strings.Split(repoURL, orgName+"/")
	if len(parts) < 2 {
		return "", fmt.Errorf("error: unable to parse Git repository URL: %v", repoURL)
	}
	return parts[1], nil
}

// GetRepoAndOrgFromURL returns both the github org and repository name from a given github URL
// Format must be of the form: <github-domain>/owner/repository(.git)
// If .git is appended to the end, it will be removed from the returned repo name
func GetRepoAndOrgFromURL(repoURL string) (string, string, error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("error: invalid URL: %v", repoURL)
	}

	// The URL Path should contain the org and repo name in the form: orgname/reponame.
	parts := strings.Split(parsedURL.Path, "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("error: unable to parse Git repository URL: %v", repoURL)
	}
	orgName := parts[1]
	if orgName == "" {
		return "", "", fmt.Errorf("error: unable to retrieve organization name from URL: %v", repoURL)
	}
	repoName := strings.Split(parts[2], ".git")[0]
	if repoName == "" {
		return "", "", fmt.Errorf("error: unable to retrieve repository name from URL: %v", repoURL)
	}
	return repoName, orgName, nil
}

// GetDefaultBranchFromURL returns the default branch of a given repoURL
func GetDefaultBranchFromURL(repoURL string, client *github.Client, ctx context.Context) (string, error) {
	repoName, orgName, err := GetRepoAndOrgFromURL(repoURL)
	if err != nil {
		return "", err
	}

	repo, _, err := client.Repositories.Get(ctx, orgName, repoName)
	if err != nil || repo == nil {
		return "", fmt.Errorf("failed to get repo %s under %s, error: %v", repoName, orgName, err)
	}

	return *repo.DefaultBranch, nil
}

// GetBranchFromURL returns the requested branch of a given repoURL
func GetBranchFromURL(repoURL string, client *github.Client, ctx context.Context, branchName string) (*github.Branch, error) {
	repoName, orgName, err := GetRepoAndOrgFromURL(repoURL)
	if err != nil {
		return nil, err
	}

	branch, _, err := client.Repositories.GetBranch(ctx, orgName, repoName, branchName, false)
	if err != nil || branch == nil {
		return nil, fmt.Errorf("failed to get branch %s from repo %s under %s, error: %v", branchName, repoName, orgName, err)
	}

	return branch, nil
}

// GetLatestCommitSHAFromRepository gets the latest Commit SHA from the repository
func GetLatestCommitSHAFromRepository(client *github.Client, ctx context.Context, repoName string, orgName string, branch string) (string, error) {
	commitSHA, _, err := client.Repositories.GetCommitSHA1(ctx, orgName, repoName, branch, "")
	if err != nil {
		return "", err
	}
	return commitSHA, nil
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

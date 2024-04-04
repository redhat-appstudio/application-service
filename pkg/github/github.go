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
	"sync"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/go-github/v59/github"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/redhat-appstudio/application-service/pkg/util"
)

const AppStudioAppDataOrg = "redhat-appstudio-appdata"

// GitHubClient represents a Go-GitHub client, along with the name of the GitHub token that was used to initialize it
type GitHubClient struct {
	TokenName          string
	Token              string
	Client             *github.Client
	SecondaryRateLimit SecondaryRateLimit
	PrimaryRateLimited bool // flag to denote if the token has been near primary rate limited
}

type SecondaryRateLimit struct {
	isLimitReached bool
	mu             sync.Mutex
}

type ContextKey string

const (
	GHClientKey ContextKey = "ghClient"
)

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

func (g *GitHubClient) GenerateNewRepository(ctx context.Context, orgName string, repoName string, description string) (string, error) {
	isPrivate := false
	appStudioAppDataURL := "https://github.com/" + orgName + "/"
	metrics.GitOpsRepoCreationTotalReqs.Inc()
	r := &github.Repository{Name: &repoName, Private: &isPrivate, Description: &description}
	_, resp, err := g.Client.Repositories.Create(ctx, orgName, r)

	if resp != nil && 500 <= resp.StatusCode && resp.StatusCode <= 599 {
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

// GetGitStatus returns the status of the Git API with a simple noop call
func (g *GitHubClient) GetGitStatus(ctx context.Context) (bool, error) {
	quote, response, err := g.Client.Zen(ctx)
	if err == nil && response != nil && response.StatusCode >= 200 && response.StatusCode <= 299 && quote != "" {
		return true, nil
	}
	return false, err
}

// GetDefaultBranchFromURL returns the default branch of a given repoURL
func (g *GitHubClient) GetDefaultBranchFromURL(repoURL string, ctx context.Context) (string, error) {
	repoName, orgName, err := GetRepoAndOrgFromURL(repoURL)
	if err != nil {
		return "", err
	}

	repo, _, err := g.Client.Repositories.Get(ctx, orgName, repoName)
	if err != nil || repo == nil {
		return "", fmt.Errorf("failed to get repo %s under %s, error: %v", repoName, orgName, err)
	}

	return *repo.DefaultBranch, nil
}

// GetBranchFromURL returns the requested branch of a given repoURL
func (g *GitHubClient) GetBranchFromURL(repoURL string, ctx context.Context, branchName string) (*github.Branch, error) {
	repoName, orgName, err := GetRepoAndOrgFromURL(repoURL)
	if err != nil {
		return nil, &GitHubUserErr{Err: err.Error()}
	}

	branch, _, err := g.Client.Repositories.GetBranch(ctx, orgName, repoName, branchName, false)
	if err != nil || branch == nil {
		return nil, &GitHubSystemErr{Err: fmt.Sprintf("failed to get branch %s from repo %s under %s, error: %v", branchName, repoName, orgName, err)}
	}

	return branch, nil
}

// GetLatestCommitSHAFromRepository gets the latest Commit SHA from the repository
func (g *GitHubClient) GetLatestCommitSHAFromRepository(ctx context.Context, repoName string, orgName string, branch string) (string, error) {
	commitSHA, _, err := g.Client.Repositories.GetCommitSHA1(ctx, orgName, repoName, branch, "")
	if err != nil {
		return "", err
	}
	return commitSHA, nil
}

// Delete Repository takes in the given repository URL and attempts to delete it
func (g *GitHubClient) DeleteRepository(ctx context.Context, orgName string, repoName string) error {
	// Retrieve just the repository name from the URL
	_, err := g.Client.Repositories.Delete(ctx, orgName, repoName)
	if err != nil {
		return err
	}
	return nil
}

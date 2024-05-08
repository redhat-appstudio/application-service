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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

func TestGenerateNewRepository(t *testing.T) {

	ctx := context.WithValue(context.Background(), GHClientKey, "mock")
	ctxNoClient := context.Background()
	ctxClientDoesNotExist := context.WithValue(context.Background(), GHClientKey, "does-not-exist")

	prometheus.MustRegister(metrics.GitOpsRepoCreationTotalReqs, metrics.GitOpsRepoCreationSucceeded, metrics.GitOpsRepoCreationFailed)
	tests := []struct {
		name                   string
		ctx                    context.Context
		repoName               string
		orgName                string
		want                   string
		wantErr                bool
		numReposCreated        int //this represents the cumulative counts of metrics assuming the tests run in order
		numReposCreationFailed int //this represents the cumulative counts of metrics assuming the tests run in order
	}{
		{
			name:                   "Simple repo name",
			ctx:                    ctx,
			repoName:               "test-repo-1",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/test-repo-1",
			wantErr:                false,
			numReposCreated:        1,
			numReposCreationFailed: 0,
		},
		{
			name:                   "Repo creation fails due to server error",
			ctx:                    ctx,
			repoName:               "test-server-error-response",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/test-error-response",
			wantErr:                true,
			numReposCreated:        1,
			numReposCreationFailed: 1,
		},
		{
			name:                   "Repo creation fails due to server error, failed metric count increases",
			ctx:                    ctx,
			repoName:               "test-server-error-response-2",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/test-error-response-2",
			wantErr:                true,
			numReposCreated:        1,
			numReposCreationFailed: 2,
		},
		{
			name:                   "Repo creation fails due to user error, metric counts should not increase",
			ctx:                    ctx,
			repoName:               "test-user-error-response",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/test-user-error-response",
			wantErr:                true,
			numReposCreated:        1,
			numReposCreationFailed: 2,
		},
		{
			name:                   "Repo creation fails due to secondary rate limit",
			ctx:                    ctx,
			repoName:               "secondary-rate-limit",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/secondary-rate-limit",
			wantErr:                true,
			numReposCreated:        1,
			numReposCreationFailed: 2,
		},
		{
			name:                   "Secondary rate limit callback fails due to no token name passed in request, but should not panic",
			ctx:                    ctxNoClient,
			repoName:               "secondary-rate-limit-callback-fail",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/secondary-rate-limit",
			wantErr:                true,
			numReposCreated:        1,
			numReposCreationFailed: 2,
		},
		{
			name:                   "Secondary rate limit callback fails due to an incorrect/non-existent token name, but should not panic",
			ctx:                    ctxClientDoesNotExist,
			repoName:               "secondary-rate-limit-callback-fail",
			orgName:                "redhat-appstudio-appdata",
			want:                   "https://github.com/redhat-appstudio-appdata/secondary-rate-limit",
			wantErr:                true,
			numReposCreated:        1,
			numReposCreationFailed: 2,
		},
	}

	numTests := len(tests)
	for _, tt := range tests {
		mockedClient := GitHubClient{
			Client:    GetMockedClient(),
			TokenName: "mock",
		}
		if Clients == nil {
			Clients = make(map[string]*GitHubClient)
		}
		Clients["mock"] = &mockedClient

		// Deliberately lock the secondary rate limit object until we need to test the related fields
		Clients["mock"].SecondaryRateLimit.mu.Lock()

		t.Run(tt.name, func(t *testing.T) {
			repoURL, err := mockedClient.GenerateNewRepository(tt.ctx, tt.orgName, tt.repoName, "")

			if err != nil && tt.wantErr {
				if _, ok := err.(*ServerError); ok {
					//validate error message
					if !strings.Contains(err.Error(), "failed to create gitops repo due to error:") {
						t.Errorf("TestGenerateNewRepository() unexpected server error message: %v", err)
					}
					//when there is a server error, we should collect the metric
					assert.Equal(t, float64(tt.numReposCreated), testutil.ToFloat64(metrics.GitOpsRepoCreationSucceeded))
					assert.Equal(t, float64(tt.numReposCreationFailed), testutil.ToFloat64(metrics.GitOpsRepoCreationFailed))
				} else {
					//If it's a user error, metric counts should not increase.  The test cases should reflect that
					assert.Equal(t, float64(tt.numReposCreated), testutil.ToFloat64(metrics.GitOpsRepoCreationSucceeded))
					assert.Equal(t, float64(tt.numReposCreationFailed), testutil.ToFloat64(metrics.GitOpsRepoCreationFailed))
				}

			}
			if (err != nil) && !tt.wantErr {
				t.Errorf("TestGenerateNewRepository() unexpected error value: %v", err)

			}
			if !tt.wantErr && repoURL != tt.want {
				t.Errorf("TestGenerateNewRepository() error: expected %v got %v", tt.want, repoURL)
			}

			if tt.repoName == "secondary-rate-limit" {
				Clients["mock"].SecondaryRateLimit.mu.Unlock()
				time.Sleep(time.Second * 1)
				if !Clients["mock"].SecondaryRateLimit.isLimitReached {
					t.Errorf("TestGenerateNewRepository() error expected github client to be secondary rate limited")
				}
				time.Sleep(time.Second * 3)
				if Clients["mock"].SecondaryRateLimit.isLimitReached {
					t.Errorf("TestGenerateNewRepository() error expected github client to no longer be secondary rate limited")
				}
			}
			//verify the number of successful repos created
			assert.Equal(t, float64(tt.numReposCreated), testutil.ToFloat64(metrics.GitOpsRepoCreationSucceeded))
			assert.Equal(t, float64(tt.numReposCreationFailed), testutil.ToFloat64(metrics.GitOpsRepoCreationFailed))
		})
	}

	assert.Equal(t, float64(numTests), testutil.ToFloat64(metrics.GitOpsRepoCreationTotalReqs))
}

func TestDeleteRepository(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		orgName  string
		wantErr  bool
	}{
		{
			name:     "Simple repo url",
			repoName: "test-repo-1",
			orgName:  "redhat-appstudio-appdata",
			wantErr:  false,
		},
		{
			name:     "Invalid repo name",
			repoName: "https://github.com/invalid/url",
			orgName:  "redhat-appstudio-appdata",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		mockedClient := GitHubClient{
			Client: GetMockedClient(),
		}

		t.Run(tt.name, func(t *testing.T) {
			err := mockedClient.DeleteRepository(context.Background(), tt.orgName, tt.repoName)

			if tt.wantErr != (err != nil) {
				t.Errorf("TestDeleteRepository() error: expected %v, got %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		orgName string
		want    string
		wantErr bool
	}{
		{
			name:    "Simple repo url",
			repoURL: "https://github.com/redhat-appstudio-appdata/test-repo-1",
			orgName: "redhat-appstudio-appdata",
			want:    "test-repo-1",
			wantErr: false,
		},
		{
			name:    "Simple repo url, invalid org name",
			repoURL: "https://github.com/redhat-appstudio-appdata/test-repo-1",
			orgName: "fakeorg",
			want:    "test-repo-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoName, err := GetRepoNameFromURL(tt.repoURL, tt.orgName)

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetRepoNameFromURL() error: expected an error to be returned")
			}

			if !tt.wantErr && (repoName != tt.want) {
				t.Errorf("TestGetRepoNameFromURL() error: expected %v got %v", tt.want, repoName)
			}
		})
	}
}

func TestGetRepoAndOrgFromURL(t *testing.T) {
	tests := []struct {
		name          string
		repoURL       string
		wantRepo      string
		wantOrg       string
		wantErr       bool
		wantErrString string
	}{
		{
			name:     "Simple repo url",
			repoURL:  "https://github.com/redhat-appstudio-appdata/test-repo-1",
			wantRepo: "test-repo-1",
			wantOrg:  "redhat-appstudio-appdata",
			wantErr:  false,
		},
		{
			name:     "Repo url with .git",
			repoURL:  "https://github.com/redhat-appstudio-appdata/test-repo-1.git",
			wantRepo: "test-repo-1",
			wantOrg:  "redhat-appstudio-appdata",
			wantErr:  false,
		},
		{
			name:     "Repo url without scheme",
			repoURL:  "github.com/redhat-appstudio-appdata/test-repo-1",
			wantRepo: "test-repo-1",
			wantOrg:  "redhat-appstudio-appdata",
			wantErr:  false,
		},
		{
			name:          "Invalid repo url",
			repoURL:       "github.comasdfsdfsafd",
			wantErr:       true,
			wantErrString: "error: unable to parse Git repository URL",
		},
		{
			name:          "Invalid repo url, with partial path",
			repoURL:       "github.com/asdfsdfsafd",
			wantErr:       true,
			wantErrString: "error: unable to parse Git repository URL",
		},
		{
			name:          "Invalid repo url, with too many paths",
			repoURL:       "github.com/asdfsdfsafd/another/another/path",
			wantErr:       true,
			wantErrString: "error: unable to parse Git repository URL",
		},
		{
			name:          "Unparseable URL",
			repoURL:       "http://github.com/?org\nrepo",
			wantErr:       true,
			wantErrString: "error: invalid URL",
		},
		{
			name:          "Unparseable organization name",
			repoURL:       "https://github.com//test",
			wantErr:       true,
			wantErrString: "error: unable to retrieve organization name from URL",
		},
		{
			name:          "Unparseable repository name",
			repoURL:       "https://github.com/organization/",
			wantErr:       true,
			wantErrString: "error: unable to retrieve repository name from URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoName, orgName, err := GetRepoAndOrgFromURL(tt.repoURL)

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetRepoAndOrgFromURL() error: expected an error to be returned")
			}

			if tt.wantErr {
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.wantErrString) {
					t.Errorf("TestGetRepoAndOrgFromURL() error: expected error message %v got %v", tt.wantErrString, errMsg)
				}
			}

			if !tt.wantErr && (repoName != tt.wantRepo) {
				t.Errorf("TestGetRepoAndOrgFromURL() error: expected %v got %v", tt.wantRepo, repoName)
			}

			if !tt.wantErr && (orgName != tt.wantOrg) {
				t.Errorf("TestGetRepoAndOrgFromURL() error: expected %v got %v", tt.wantOrg, orgName)
			}
		})
	}
}

func TestGetGitStatus(t *testing.T) {
	tests := []struct {
		name          string
		repoName      string
		orgName       string
		wantAvailable bool
		wantErr       bool
	}{
		{
			name:          "Simple repo name",
			repoName:      "test-repo-1",
			orgName:       "redhat-appstudio-appdata",
			wantAvailable: true,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		mockedClient := GitHubClient{
			Client: GetMockedClient(),
		}

		t.Run(tt.name, func(t *testing.T) {
			isGitAvailable, err := mockedClient.GetGitStatus(context.Background())

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetGitStatus() unexpected error value: %v", err)
			}
			if !tt.wantErr && isGitAvailable != tt.wantAvailable {
				t.Errorf("TestGetGitStatus() error: expected %v got %v", tt.wantAvailable, isGitAvailable)
			}
		})
	}
}

func TestGetLatestCommitSHAFromRepository(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		orgName  string
		want     string
		wantErr  bool
	}{
		{
			name:     "Simple repo name",
			repoName: "test-repo-1",
			orgName:  "redhat-appstudio-appdata",
			want:     "ca82a6dff817ec66f44342007202690a93763949",
			wantErr:  false,
		},
		{
			name:     "Simple repo name",
			repoName: "test-error-response",
			orgName:  "some-org",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		mockedClient := GitHubClient{
			Client: GetMockedClient(),
		}

		t.Run(tt.name, func(t *testing.T) {
			commitSHA, err := mockedClient.GetLatestCommitSHAFromRepository(context.Background(), tt.orgName, tt.repoName, "main")

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetLatestCommitSHAFromRepository() unexpected error value: %v", err)
			}
			if !tt.wantErr && commitSHA != tt.want {
				t.Errorf("TestGetLatestCommitSHAFromRepository() error: expected %v got %v", tt.want, commitSHA)
			}
		})
	}
}

func TestGetDefaultBranchFromURL(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		want    string
		wantErr bool
	}{
		{
			name:    "repo with main as default branch",
			repoURL: "https://github.com/redhat-appstudio-appdata/test-repo-1",
			want:    "main",
			wantErr: false,
		},
		{
			name:    "repo with master as default branch",
			repoURL: "https://github.com/redhat-appstudio-appdata/test-repo-2.git",
			want:    "master",
			wantErr: false,
		},
		{
			name:    "Simple repo name",
			repoURL: "https://github.com/some-org/test-error-response",
			wantErr: true,
		},
		{
			name:    "Unparseable URL",
			repoURL: "http://github.com/?org\nrepo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		mockedClient := GitHubClient{
			Client: GetMockedClient(),
		}

		t.Run(tt.name, func(t *testing.T) {
			defaultBranch, err := mockedClient.GetDefaultBranchFromURL(tt.repoURL, context.Background())

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetDefaultBranchFromURL() unexpected error value: %v", err)
			}
			if !tt.wantErr && defaultBranch != tt.want {
				t.Errorf("TestGetDefaultBranchFromURL() error: expected %v got %v", tt.want, defaultBranch)
			}
		})
	}
}

func TestGetBranchFromURL(t *testing.T) {
	tests := []struct {
		name       string
		repoURL    string
		branchName string
		wantErr    bool
	}{
		{
			name:       "repo with main as default branch",
			repoURL:    "https://github.com/redhat-appstudio-appdata/test-repo-1",
			branchName: "main",
			wantErr:    false,
		},
		{
			name:       "repo with master as default branch",
			repoURL:    "https://github.com/redhat-appstudio-appdata/test-repo-2.git",
			branchName: "master",
			wantErr:    false,
		},
		{
			name:       "Simple repo name",
			repoURL:    "https://github.com/redhat-appstudio-appdata/test-repo-2.git",
			branchName: "main",
			wantErr:    true,
		},
		{
			name:    "Unparseable URL",
			repoURL: "http://github.com/?org\nrepo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		mockedClient := GitHubClient{
			Client: GetMockedClient(),
		}

		t.Run(tt.name, func(t *testing.T) {
			branch, err := mockedClient.GetBranchFromURL(tt.repoURL, context.Background(), tt.branchName)

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetBranchFromURL() unexpected error value: %v, branch %v", err, branch)
			}
			if !tt.wantErr && *branch.Name != tt.branchName {
				t.Errorf("TestGetBranchFromURL() error: expected %v got %v", tt.branchName, *branch.Name)
			}
		})
	}
}

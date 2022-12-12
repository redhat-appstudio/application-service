//
// Copyright 2021 Red Hat, Inc.
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
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/redhat-appstudio/application-service/pkg/util"
)

func TestGenerateNewRepositoryName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		namespace   string
		clusterName string
		want        string
	}{
		{
			name:        "Simple display name, no spaces",
			displayName: "PetClinic",
			namespace:   "default",
			clusterName: "root:test-workspace",
			want:        "petclinic-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := util.SanitizeName(tt.displayName)
			h := sha256.New()
			h.Write([]byte(tt.clusterName + tt.namespace))
			namespaceClusterHash := base64.URLEncoding.EncodeToString(h.Sum(nil))[0:5]
			generatedRepo := GenerateNewRepositoryName(tt.displayName, tt.namespace, tt.clusterName)

			if !strings.Contains(generatedRepo, sanitizedName) || !strings.Contains(generatedRepo, namespaceClusterHash) {
				t.Errorf("TestSanitizeDisplayName() error: expected %v got %v", tt.want, sanitizedName)
			}
		})
	}
}

func TestGenerateNewRepository(t *testing.T) {
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
			want:     "https://github.com/redhat-appstudio-appdata/test-repo-1",
			wantErr:  false,
		},
		{
			name:     "Repo creation fails",
			repoName: "test-error-response",
			orgName:  "redhat-appstudio-appdata",
			want:     "https://github.com/redhat-appstudio-appdata/test-repo-1",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		mockedClient := GetMockedClient()

		t.Run(tt.name, func(t *testing.T) {
			repoURL, err := GenerateNewRepository(mockedClient, context.Background(), tt.orgName, tt.repoName, "")

			if (err != nil) && !tt.wantErr {
				t.Errorf("TestGenerateNewRepository() unexpected error value: %v", err)
			}
			if !tt.wantErr && repoURL != tt.want {
				t.Errorf("TestGenerateNewRepository() error: expected %v got %v", tt.want, repoURL)
			}
		})
	}
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
		mockedClient := GetMockedClient()

		t.Run(tt.name, func(t *testing.T) {
			err := DeleteRepository(mockedClient, context.Background(), tt.orgName, tt.repoName)

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
		mockedClient := GetMockedClient()

		t.Run(tt.name, func(t *testing.T) {
			commitSHA, err := GetLatestCommitSHAFromRepository(mockedClient, context.Background(), tt.orgName, tt.repoName, "main")

			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetLatestCommitSHAFromRepository() unexpected error value: %v", err)
			}
			if !tt.wantErr && commitSHA != tt.want {
				t.Errorf("TestGetLatestCommitSHAFromRepository() error: expected %v got %v", tt.want, commitSHA)
			}
		})
	}
}

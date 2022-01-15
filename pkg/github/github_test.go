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
	"strings"
	"testing"

	"github.com/redhat-appstudio/application-service/pkg/util"
)

func TestGenerateNewRepositoryName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		namespace   string
		want        string
	}{
		{
			name:        "Simple display name, no spaces",
			displayName: "PetClinic",
			namespace:   "default",
			want:        "petclinic-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := util.SanitizeDisplayName(tt.displayName)
			generatedRepo := GenerateNewRepositoryName(tt.displayName, tt.namespace)

			if !strings.Contains(generatedRepo, sanitizedName) {
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
		wantErr  error
	}{
		{
			name:     "Simple repo name",
			repoName: "test-repo-1",
			orgName:  "redhat-appstudio-appdata",
			want:     "https://github.com/redhat-appstudio-appdata/test-repo-1",
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		mockedClient := GetMockedClient()

		t.Run(tt.name, func(t *testing.T) {
			repoURL, err := GenerateNewRepository(mockedClient, context.Background(), tt.orgName, tt.repoName, "")

			if err != tt.wantErr {
				t.Errorf("TestGenerateNewRepository() error: expected %v got %v", tt.wantErr, err)
			} else if repoURL != tt.want {
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

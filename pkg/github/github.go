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
	"fmt"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/go-github/v41/github"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"golang.org/x/oauth2"
)

const AppStudioAppDataOrg = "redhat-appstudio-appdata"

func NewGithubClient(token string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func GenerateNewRepositoryName(displayName string, namespace string) string {
	sanitizedName := util.SanitizeName(displayName)

	repoName := sanitizedName + "-" + namespace + "-" + util.SanitizeName(gofakeit.Verb()) + "-" + util.SanitizeName(gofakeit.Noun())
	return repoName
}

func GenerateNewRepository(client *github.Client, ctx context.Context, orgName string, repoName string, description string) (string, error) {
	isPrivate := true
	appStudioAppDataURL := "https://github.com/" + orgName + "/"

	r := &github.Repository{Name: &repoName, Private: &isPrivate, Description: &description}
	_, _, err := client.Repositories.Create(ctx, orgName, r)
	if err != nil {
		return "", err
	}
	repoURL := appStudioAppDataURL + repoName
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

// Delete Repository takes in the given repository URL and attempts to delete it
func DeleteRepository(client *github.Client, ctx context.Context, orgName string, repoName string) error {
	// Retrieve just the repository name from the URL
	_, err := client.Repositories.Delete(ctx, orgName, repoName)
	if err != nil {
		return err
	}
	return nil
}

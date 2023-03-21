//
// Copyright 2023 Red Hat, Inc.
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
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/gofri/go-github-ratelimit/github_ratelimit"
	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"
)

type GitHubToken interface {
	GetNewGitHubClient() (*github.Client, error)
}

type GitHubTokenClient struct {
}

var Tokens []string

// ParseGitHubTokens parses all of the possible GitHub tokens available to HAS and makes them available within the "github" package
// This function should *only* be called once: at operator startup.
func ParseGitHubTokens() error {
	githubToken := os.Getenv("GITHUB_AUTH_TOKEN")
	githubTokenList := os.Getenv("GITHUB_TOKEN_LIST")
	if githubToken == "" && githubTokenList == "" {
		return fmt.Errorf("no GitHub tokens were provided. Either GITHUB_TOKENS or GITHUB_AUTH_TOKEN (legacy) must be set")
	}

	var tokenList []string
	if githubToken != "" {
		tokenList = append(tokenList, githubToken)
	}
	if githubTokenList != "" {
		splitTokenList := strings.Split(githubTokenList, ",")
		tokenList = append(tokenList, splitTokenList...)
	}

	Tokens = tokenList

	return nil
}

// getRandomToken randomly retrieves
func getRandomToken() (string, error) {
	if len(Tokens) == 0 {
		return "", fmt.Errorf("no GitHub tokens initialized")
	}
	if len(Tokens) == 1 {
		return Tokens[0], nil
	}
	return Tokens[rand.Intn(len(Tokens)-1)], nil
}

// GetNewGitHubClient intializes a new Go-GitHub client from a randomly selecte GitHub token available to HAS
// If an error is encountered retrieveing the token, or initializing the client, an error is returned
func (g GitHubTokenClient) GetNewGitHubClient() (*github.Client, error) {
	ghToken, err := getRandomToken()
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ghToken})
	tc := oauth2.NewClient(context.Background(), ts)
	rateLimiter, err := github_ratelimit.NewRateLimitWaiterClient(tc.Transport, github_ratelimit.WithSingleSleepLimit(time.Minute, nil))
	if err != nil {
		return nil, err
	}
	client := github.NewClient(rateLimiter)

	return client, nil
}

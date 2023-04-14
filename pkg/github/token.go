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
	GetNewGitHubClient(token string) (GitHubClient, error)
}

type GitHubTokenClient struct {
}

// Tokens is mapping of token names to GitHub tokens
var Tokens map[string]string

// ParseGitHubTokens parses all of the possible GitHub tokens available to HAS and makes them available within the "github" package
// This function should *only* be called once: at operator startup.
func ParseGitHubTokens() error {
	githubToken := os.Getenv("GITHUB_AUTH_TOKEN")
	githubTokenList := os.Getenv("GITHUB_TOKEN_LIST")
	if githubToken == "" && githubTokenList == "" {
		return fmt.Errorf("no GitHub tokens were provided. Either GITHUB_TOKEN_LIST or GITHUB_AUTH_TOKEN (legacy) must be set")
	}

	Tokens = make(map[string]string)
	if githubToken != "" {
		// The old token format, stored in 'GITHUB_AUTH_TOKEN', didn't require a key/'name' for the token
		// So use the key 'GITHUB_AUTH_TOKEN' for it
		Tokens["GITHUB_AUTH_TOKEN"] = githubToken
	}

	// Parse any tokens passed in through the 'GITHUB_TOKEN_LIST' environment variable
	// e.g. GITHUB_TOKEN_LIST=token1:ghp_faketoken,token2:ghp_anothertoken
	if githubTokenList != "" {
		// Each token key-value pair is separated by a comma, so split the string based on commas and loop over each key-value pair
		tokenKeyValuePairs := strings.Split(githubTokenList, ",")
		for _, tokenKeyValuePair := range tokenKeyValuePairs {
			// Each token key-value pair is separated by a colon, so split the key-value pair and
			// If the key-value pair doesn't split cleanly (i.e. only two strings returned), return an error
			// If the key has already been added, return an error
			splitTokenKeyValuePair := strings.Split(tokenKeyValuePair, ":")
			if len(splitTokenKeyValuePair) != 2 {
				return fmt.Errorf("unable to parse github token from key-value pair. Please ensure the GitHub secret is formatted correctly according to the documentation")
			}
			tokenKey := splitTokenKeyValuePair[0]
			tokenValue := splitTokenKeyValuePair[1]

			if Tokens[tokenKey] != "" {
				return fmt.Errorf("a token with the key '%s' already exists. Each token must have a unique key", tokenKey)
			}
			Tokens[tokenKey] = tokenValue
		}
	}

	return nil
}

// getRandomToken randomly retrieves a token from all of the tokens available to HAS
// It returns the token and the name/key of the token
func getRandomToken() (string, string, error) {
	if len(Tokens) == 0 {
		return "", "", fmt.Errorf("no GitHub tokens initialized")
	}
	/* #nosec G404 -- not used for cryptographic purposes*/
	index := rand.Intn(len(Tokens))

	i := 0
	var tokenName, token string
	for k, v := range Tokens {
		if i == index {
			tokenName = k
			token = v
		}
		i++
	}
	return token, tokenName, nil
}

// GetNewGitHubClient intializes a new Go-GitHub client
// If a token is passed in (non-empty string) it will use that token for the GitHub client
// If no token is passed in (empty string), a token will be randomly selected by HAS.
// It returns the GitHub client, and (if a token was randomly selected) the name of the token used for the client
// If an error is encountered retrieving the token, or initializing the client, an error is returned
func (g GitHubTokenClient) GetNewGitHubClient(token string) (GitHubClient, error) {
	var ghToken, ghTokenName string
	var err error
	if token == "" {
		ghToken, ghTokenName, err = getRandomToken()
		if err != nil {
			return GitHubClient{}, err
		}
	} else {
		ghToken = token
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ghToken})
	tc := oauth2.NewClient(context.Background(), ts)
	rateLimiter, err := github_ratelimit.NewRateLimitWaiterClient(tc.Transport, github_ratelimit.WithSingleSleepLimit(time.Minute, nil))
	if err != nil {
		return GitHubClient{}, err
	}
	client := github.NewClient(rateLimiter)
	githubClient := GitHubClient{
		TokenName: ghTokenName,
		Token:     ghToken,
		Client:    client,
	}

	return githubClient, nil
}

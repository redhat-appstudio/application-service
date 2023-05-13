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
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/application-service/pkg/metrics"

	"github.com/gofri/go-github-ratelimit/github_ratelimit"
	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"

	ctrl "sigs.k8s.io/controller-runtime"
)

type GitHubToken interface {
	GetNewGitHubClient(token string) (*GitHubClient, error)
}

type GitHubTokenClient struct {
}

// Tokens is mapping of token names to GitHub tokens
var Clients map[string]*GitHubClient

// ParseGitHubTokens parses all of the possible GitHub tokens available to HAS and makes them available within the "github" package
// This function should *only* be called once: at operator startup.
func ParseGitHubTokens() error {
	githubToken := os.Getenv("GITHUB_AUTH_TOKEN")
	githubTokenList := os.Getenv("GITHUB_TOKEN_LIST")
	if githubToken == "" && githubTokenList == "" {
		return fmt.Errorf("no GitHub tokens were provided. Either GITHUB_TOKEN_LIST or GITHUB_AUTH_TOKEN (legacy) must be set")
	}

	Clients = make(map[string]*GitHubClient)
	if githubToken != "" {
		// The old token format, stored in 'GITHUB_AUTH_TOKEN', didn't require a key/'name' for the token
		// So use the key 'GITHUB_AUTH_TOKEN' for it
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
		tc := oauth2.NewClient(context.Background(), ts)
		token, err := createGitHubClientFromToken(&tc.Transport, githubToken, "GITHUB_AUTH_TOKEN")
		if err != nil {
			return err
		}
		Clients["GITHUB_AUTH_TOKEN"] = token
	}

	// Parse any tokens passed in through the 'GITHUB_TOKEN_LIST' environment variable
	// e.g. GITHUB_TOKEN_LIST=token1:ghp_faketoken,token2:ghp_anothertoken
	//emptyGitHubClient := GitHubClient{}
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

			if Clients[tokenKey] != nil {
				return fmt.Errorf("a token with the key '%s' already exists. Each token must have a unique key", tokenKey)
			}

			ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokenValue})
			tc := oauth2.NewClient(context.Background(), ts)
			token, err := createGitHubClientFromToken(&tc.Transport, tokenValue, tokenKey)
			if err != nil {
				return err
			}
			Clients[tokenKey] = token
		}
	}

	return nil
}

// getRandomClient randomly retrieves a token from all of the tokens available to HAS
// It returns the token and the name/key of the token
func getRandomClient(clientPool map[string]*GitHubClient) (*GitHubClient, error) {
	if len(clientPool) == 0 {
		return nil, fmt.Errorf("no GitHub tokens available")
	}
	/* #nosec G404 -- not used for cryptographic purposes*/
	index := rand.Intn(len(clientPool))

	i := 0
	var ghClient *GitHubClient
	for _, v := range clientPool {
		if i == index {
			ghClient = v
		}
		i++
	}

	// Check the Primary rate limit
	rl, _, err := ghClient.Client.RateLimits(context.Background())
	if err != nil {
		return nil, err
	}
	if rl != nil && (rl.Core != nil && rl.Core.Remaining < 10) || (rl.Search != nil && rl.Search.Remaining < 2) {
		newClientPool := make(map[string]*GitHubClient)
		for k, v := range clientPool {
			if k != ghClient.TokenName {
				newClientPool[k] = v
			}
		}
		metrics.TokenPoolCounter.With(prometheus.Labels{"rateLimited": "primary", "tokenName": ghClient.TokenName, "tokensRemaining": strconv.Itoa(len(newClientPool))}).Inc()
		return getRandomClient(newClientPool)
	}

	// Check the secondary rate limit
	var isSecondaryRl bool
	ghClient.SecondaryRateLimit.mu.Lock()
	isSecondaryRl = ghClient.SecondaryRateLimit.isLimitReached
	ghClient.SecondaryRateLimit.mu.Unlock()

	if isSecondaryRl {
		newClientPool := make(map[string]*GitHubClient)
		for k, v := range clientPool {
			if k != ghClient.TokenName {
				newClientPool[k] = v
			}
		}
		metrics.TokenPoolCounter.With(prometheus.Labels{"rateLimited": "secondary", "tokenName": ghClient.TokenName, "tokensRemaining": strconv.Itoa(len(newClientPool))}).Inc()
		return getRandomClient(newClientPool)
	}
	return ghClient, nil
}

// GetNewGitHubClient returns a Go-GitHub client
// If a token is passed in (non-empty string) it will use that token for the GitHub client
// If no token is passed in (empty string), a token will be randomly selected by HAS.
// It returns the GitHub client, and (if a token was randomly selected) the name of the token used for the client
// If an error is encountered retrieving the token, or initializing the client, an error is returned
func (g GitHubTokenClient) GetNewGitHubClient(token string) (*GitHubClient, error) {
	var ghToken string
	if token != "" {
		ghToken = token
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ghToken})
		tc := oauth2.NewClient(context.Background(), ts)
		ghClient, err := createGitHubClientFromToken(&tc.Transport, ghToken, "")
		if err != nil {
			return nil, err
		}
		return ghClient, nil
	} else {
		ghClient, err := getRandomClient(Clients)
		if err != nil {
			return nil, err
		}
		return ghClient, nil
	}
}

func createGitHubClientFromToken(roundTripper *http.RoundTripper, ghToken string, ghTokenName string) (*GitHubClient, error) {
	rateLimiter, err := github_ratelimit.NewRateLimitWaiterClient(*roundTripper, github_ratelimit.WithSingleSleepLimit(0, rateLimitCallBackfunc))

	if err != nil {
		return nil, err
	}
	client := github.NewClient(rateLimiter)
	githubClient := GitHubClient{
		TokenName: ghTokenName,
		Token:     ghToken,
		Client:    client,
	}

	return &githubClient, nil
}

func rateLimitCallBackfunc(cbContext *github_ratelimit.CallbackContext) {
	// Retrieve the request's context and get the client name from it
	// Use the client name to lookup the client pointer
	req := *cbContext.Request
	reqCtx := req.Context()
	log := ctrl.LoggerFrom(reqCtx)
	ghClientNameObj := reqCtx.Value(GHClientKey)
	if ghClientNameObj == nil {
		// The GitHub Client should never be nil - it must always be set before we access the GH API
		// But if it is nil, returning prematurely is preferable to panicking
		log.Error(fmt.Errorf("a Go-GitHub client name was not set in GitHub API request, cannot execute secondary rate limit callback"), "")
		return
	}
	ghClientName := ghClientNameObj.(string)
	ghClient := Clients[ghClientName]
	if ghClient == nil {
		// Likewise, the GitHub client should never be nil, it's directly set from the calling Go-GitHub client
		// But if it is nil, returning prematurely is preferable to panicking.
		log.Error(fmt.Errorf("a Go-GitHub client with the name %v as set in the GitHub API request does not exist, cannot execute secondary rate limit callback", ghClientName), "")
		return
	}

	// Start a goroutine that marks the given client as rate limited and sleeps for 'TotalSleepTime'
	go func(client *GitHubClient) {
		client.SecondaryRateLimit.mu.Lock()
		client.SecondaryRateLimit.isLimitReached = true
		client.SecondaryRateLimit.mu.Unlock()

		// Sleep until the rate limit is over
		time.Sleep(time.Until(*cbContext.SleepUntil))
		client.SecondaryRateLimit.mu.Lock()
		client.SecondaryRateLimit.isLimitReached = false
		client.SecondaryRateLimit.mu.Unlock()
	}(ghClient)

}

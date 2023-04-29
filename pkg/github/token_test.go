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
	"os"
	"reflect"
	"testing"
	"time"
)

func TestParseGitHubTokens(t *testing.T) {
	tests := []struct {
		name               string
		githubTokenEnv     string
		githubTokenListEnv string
		want               map[string]*GitHubClient
		wantErr            bool
	}{
		{
			name:    "No tokens set",
			wantErr: true,
		},
		{
			name:           "Only one token, stored in GITHUB_AUTH_TOKEN",
			githubTokenEnv: "some_token",
			want: map[string]*GitHubClient{
				"GITHUB_AUTH_TOKEN": {
					TokenName: "GITHUB_AUTH_TOKEN",
					Token:     "some_token",
				},
			},
		},
		{
			name:               "Only one token, stored in GITHUB_TOKEN_LIST",
			githubTokenListEnv: "token1:list_token",
			want: map[string]*GitHubClient{
				"token1": {
					TokenName: "token1",
					Token:     "list_token",
				},
			},
		},
		{
			name:               "Two tokens, one each stored in GITHUB_AUTH_TOKEN and GITHUB_TOKEN_LIST",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token",
			want: map[string]*GitHubClient{
				"GITHUB_AUTH_TOKEN": {
					TokenName: "GITHUB_AUTH_TOKEN",
					Token:     "some_token",
				},
				"token1": {
					TokenName: "token1",
					Token:     "list_token",
				},
			},
		},
		{
			name:               "Multiple tokens",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token,token2:another_token,token3:third_token",
			want: map[string]*GitHubClient{
				"GITHUB_AUTH_TOKEN": {
					TokenName: "GITHUB_AUTH_TOKEN",
					Token:     "some_token",
				},
				"token1": {
					TokenName: "token1",
					Token:     "list_token",
				},
				"token2": {
					TokenName: "token2",
					Token:     "another_token",
				},
				"token3": {
					TokenName: "token3",
					Token:     "third_token",
				},
			},
		},
		{
			name:               "Error parsing tokens",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token,token2:another_token,token3",
			wantErr:            true,
		},
		{
			name:               "Error parsing tokens - invalid key separator",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token,token2:another_token:",
			wantErr:            true,
		},
		{
			name:               "Error parsing tokens - duplicate keys",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token,token1:another_token",
			wantErr:            true,
		},
		{
			name:               "Error parsing tokens - duplicate keys",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token,token1:another_token",
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("GITHUB_AUTH_TOKEN")
			os.Unsetenv("GITHUB_TOKEN_LIST")
			if tt.githubTokenEnv != "" {
				os.Setenv("GITHUB_AUTH_TOKEN", tt.githubTokenEnv)
			}
			if tt.githubTokenListEnv != "" {
				os.Setenv("GITHUB_TOKEN_LIST", tt.githubTokenListEnv)
			}

			err := ParseGitHubTokens()
			if tt.wantErr != (err != nil) {
				t.Errorf("TestParseGitHubTokens() error: unexpected error value %v", err)
			}
			if !tt.wantErr {
				for k, v := range Clients {
					client := v.Client
					tt.want[k].Client = client
				}
				if !reflect.DeepEqual(Clients, tt.want) {
					t.Errorf("TestParseGitHubTokens() error: expected %v got %v", tt.want, Clients)
				}
			}

		})
	}
}

func TestGetNewGitHubClient(t *testing.T) {
	//ghTokenClient := GitHubTokenClient{}

	//fakeToken := "ghp_faketoken"

	tests := []struct {
		name               string
		client             GitHubToken
		githubTokenEnv     string
		githubTokenListEnv string
		passedInToken      string
		wantErr            bool
	}{
		{
			name:    "Mock client",
			client:  MockGitHubTokenClient{},
			wantErr: false,
		},
		{
			name:          "Passed in token",
			client:        GitHubTokenClient{},
			passedInToken: "fake-token", // Use an empty token here instead of a fake token string.
			wantErr:       false,
		},
		{
			name:           "Empty token passed in - should error out because rate limited",
			client:         MockPrimaryRateLimitGitHubTokenClient{},
			githubTokenEnv: " ", // Use an empty token here instead of a fake token string, since we need to make a request to GH RateLimit API
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Clients = nil
			os.Unsetenv("GITHUB_AUTH_TOKEN")
			os.Unsetenv("GITHUB_TOKEN_LIST")
			if tt.githubTokenEnv != "" {
				os.Setenv("GITHUB_AUTH_TOKEN", tt.githubTokenEnv)
			}
			if tt.githubTokenListEnv != "" {
				os.Setenv("GITHUB_TOKEN_LIST", tt.githubTokenListEnv)
			}

			_ = ParseGitHubTokens()
			ghClient, err := tt.client.GetNewGitHubClient(tt.passedInToken)
			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetNewGitHubClient() error: unexpected error value %v", err)
			}
			if tt.name != "Mock client" && tt.name != "Passed in token" && !tt.wantErr {
				if ghClient == nil {
					t.Errorf("TestGetNewGitHubClient() error: did not expect GitHub Client to be nil")
				}
				if ghClient != nil && Clients[ghClient.TokenName] == nil {
					t.Errorf("TestGetNewGitHubClient() error: expected token value %v with key %v", Clients[ghClient.TokenName], ghClient.TokenName)
				}
			}

		})
	}
}

func TestGetRandomClient(t *testing.T) {
	//ghTokenClient := GitHubTokenClient{}

	//fakeToken := "ghp_faketoken"

	tests := []struct {
		name               string
		client             GitHubToken
		clientPool         map[string]*GitHubClient
		githubTokenEnv     string
		githubTokenListEnv string
		passedInToken      string
		wantErr            bool
	}{
		{
			name:       "Empty client pool - should return an error",
			clientPool: make(map[string]*GitHubClient),
			wantErr:    true,
		},
		{
			name: "primary-rate-limit",
			clientPool: map[string]*GitHubClient{
				"fake1": {
					TokenName: "fake1",
					Token:     "fake",
					Client:    GetMockedPrimaryRateLimitedClient(),
				},
			},
			wantErr: true,
		},
		{
			name: "secondary-rate-limit",
			clientPool: map[string]*GitHubClient{
				"mock": {
					TokenName: "mock",
					Token:     "fake",
					Client:    GetMockedSecondaryRateLimitedClient(),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			client, err := getRandomClient(tt.clientPool)
			if tt.wantErr != (err != nil) && tt.name != "secondary-rate-limit" {
				t.Errorf("TestGetRandomClient() error: unexpected error value %v", err)
			}
			if tt.name == "secondary-rate-limit" {
				Clients = tt.clientPool
				// Deliberately lock the secondary rate limit object until we need to test the related fields
				client.SecondaryRateLimit.mu.Lock()
				_, err := client.GenerateNewRepository(context.Background(), "test-org", "test-repo", "test description")
				if err == nil {
					t.Error("TestGetRandomClient() error: expected err not to be nil")
				}

				client.SecondaryRateLimit.mu.Unlock()
				time.Sleep(time.Second * 1)

				_, err = getRandomClient(tt.clientPool)
				if err == nil {
					t.Error("TestGetRandomClient() error: unexpected err not to be nil")
				}

			}

		})
	}
}

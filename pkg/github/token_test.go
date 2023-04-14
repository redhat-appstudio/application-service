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
	"os"
	"reflect"
	"testing"
)

func TestParseGitHubTokens(t *testing.T) {
	tests := []struct {
		name               string
		githubTokenEnv     string
		githubTokenListEnv string
		want               map[string]string
		wantErr            bool
	}{
		{
			name:    "No tokens set",
			wantErr: true,
		},
		{
			name:           "Only one token, stored in GITHUB_AUTH_TOKEN",
			githubTokenEnv: "some_token",
			want: map[string]string{
				"GITHUB_AUTH_TOKEN": "some_token",
			},
		},
		{
			name:               "Only one token, stored in GITHUB_TOKEN_LIST",
			githubTokenListEnv: "token1:list_token",
			want: map[string]string{
				"token1": "list_token",
			},
		},
		{
			name:               "Two tokens, one each stored in GITHUB_AUTH_TOKEN and GITHUB_TOKEN_LIST",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token",
			want: map[string]string{
				"GITHUB_AUTH_TOKEN": "some_token",
				"token1":            "list_token",
			},
		},
		{
			name:               "Multiple tokens",
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:list_token,token2:another_token,token3:third_token",
			want: map[string]string{
				"GITHUB_AUTH_TOKEN": "some_token",
				"token1":            "list_token",
				"token2":            "another_token",
				"token3":            "third_token",
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
				if !reflect.DeepEqual(Tokens, tt.want) {
					t.Errorf("TestParseGitHubTokens() error: expected %v got %v", tt.want, Tokens)
				}
			}

		})
	}
}

func TestGetNewGitHubClient(t *testing.T) {
	ghTokenClient := GitHubTokenClient{}

	fakeToken := "ghp_faketoken"

	tests := []struct {
		name               string
		client             GitHubToken
		githubTokenEnv     string
		githubTokenListEnv string
		passedInToken      string
		wantErr            bool
	}{
		{
			name:    "No tokens initialized, error should be returned",
			client:  ghTokenClient,
			wantErr: true,
		},
		{
			name:           "One token set, should return client",
			client:         ghTokenClient,
			githubTokenEnv: "some_token",
			wantErr:        false,
		},
		{
			name:               "Multiple tokens, should return client",
			client:             ghTokenClient,
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:another_token,token2:third_token",
			wantErr:            false,
		},
		{
			name:               "Multiple tokens, should return client",
			client:             ghTokenClient,
			githubTokenEnv:     "some_token",
			githubTokenListEnv: "token1:another_token,token:2third_token",
			wantErr:            false,
		},
		{
			name:          "One token set, should return client",
			client:        ghTokenClient,
			passedInToken: fakeToken,
			wantErr:       false,
		},
		{
			name:    "Mock client",
			client:  MockGitHubTokenClient{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Tokens = nil
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
			if tt.name != "Mock client" && !tt.wantErr {
				if Tokens[ghClient.TokenName] == "" {
					t.Errorf("TestGetNewGitHubClient() error: expected token value %v with key %v", Tokens[ghClient.TokenName], ghClient.TokenName)
				}
			}
			if tt.wantErr != (err != nil) {
				t.Errorf("TestGetNewGitHubClient() error: unexpected error value %v", err)
			}

		})
	}
}

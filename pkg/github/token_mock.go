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
	"github.com/google/go-github/v41/github"
)

type MockGitHubTokenClient struct {
}

// GetNewGitHubClient intializes a new Go-GitHub client from a randomly selecte GitHub token available to HAS
// If an error is encountered retrieveing the token, or initializing the client, an error is returned
func (g MockGitHubTokenClient) GetNewGitHubClient() (*github.Client, error) {
	return GetMockedClient(), nil
}

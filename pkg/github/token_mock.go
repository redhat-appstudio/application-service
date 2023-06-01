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

type MockGitHubTokenClient struct {
}

type MockPrimaryRateLimitGitHubTokenClient struct {
}

type MockResetPrimaryRateLimitGitHubTokenClient struct {
}

// GetNewGitHubClient returns a mocked Go-GitHub client. No actual tokens are passed in or used when this function is called
func (g MockGitHubTokenClient) GetNewGitHubClient(token string) (*GitHubClient, error) {
	fakeClients := make(map[string]*GitHubClient)
	fakeClients["fake1"] = &GitHubClient{
		TokenName: "fake1",
		Token:     token,
		Client:    GetMockedClient(),
	}
	fakeClients["fake2"] = &GitHubClient{
		TokenName: "fake2",
		Token:     "faketoken2",
		Client:    GetMockedClient(),
	}
	fakeClients["fake3"] = &GitHubClient{
		TokenName: "fake3",
		Token:     "faketoken3",
		Client:    GetMockedClient(),
	}

	return getRandomClient(fakeClients)
}

func (g MockPrimaryRateLimitGitHubTokenClient) GetNewGitHubClient(token string) (*GitHubClient, error) {
	fakeClients := make(map[string]*GitHubClient)
	fakeClients["fake1"] = &GitHubClient{
		TokenName: "fake1",
		Token:     token,
		Client:    GetMockedPrimaryRateLimitedClient(),
	}

	return getRandomClient(fakeClients)
}

func (g MockResetPrimaryRateLimitGitHubTokenClient) GetNewGitHubClient(token string) (*GitHubClient, error) {
	fakeClients := make(map[string]*GitHubClient)
	fakeClients["fake_reset"] = &GitHubClient{
		TokenName: "fake_reset",
		Token:     token,
		Client:    GetMockedResetPrimaryRateLimitedClient(),
	}

	return getRandomClient(fakeClients)
}

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
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/v52/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
)

// GetMockedClient returns a simple mocked go-github client
func GetMockedClient() *github.Client {
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetUsersByUsername,
			github.User{
				Name: github.String("testuser"),
			},
		),
		mock.WithRequestMatch(
			mock.GetUsersOrgsByUsername,
			[]github.Organization{
				{
					Name: github.String("redhat-appstudio-appdata"),
				},
			},
		),
		mock.WithRequestMatchHandler(
			mock.PostOrgsReposByOrg,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				b, _ := ioutil.ReadAll(req.Body)
				reqBody := string(b)
				// ToDo: Figure out a better way to dynamically mock errors
				if strings.Contains(reqBody, "test-error-response") || strings.Contains(reqBody, "test-server-error-response") || strings.Contains(reqBody, "test-server-error-response-2") {
					WriteError(w,
						http.StatusInternalServerError,
						"github went belly up or something",
					)
				} else if strings.Contains(reqBody, "test-user-error-response") {
					WriteError(w,
						http.StatusUnauthorized,
						"user is unauthorized",
					)
				} else if strings.Contains(reqBody, "secondary-rate-limit") {
					w.Header().Add("Retry-After", "3")
					WriteError(w,
						http.StatusForbidden,
						"secondary rate limit",
					)
				} else {
					/* #nosec G104 -- test code */
					w.Write(mock.MustMarshal(github.Repository{
						Name: github.String("test-repo-1"),
					}))
				}
			}),
		),
		mock.WithRequestMatchHandler(
			mock.DeleteReposByOwnerByRepo,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				/* #nosec G104 -- test code */
				w.Write(mock.MustMarshal(github.Repository{
					Name: github.String("test-repo-1"),
				}))
			}),
		),
		mock.WithRequestMatchHandler(
			mock.GetRateLimit,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				/* #nosec G104 -- test code */
				response := new(struct {
					Resources *github.RateLimits `json:"resources"`
				})
				response.Resources = &github.RateLimits{
					Core: &github.Rate{
						Limit:     5000,
						Remaining: 100,
					},
					Search: &github.Rate{
						Limit:     30,
						Remaining: 15,
					},
				}
				w.Write(mock.MustMarshal(response))
			}),
		),
		mock.WithRequestMatchHandler(
			mock.GetReposCommitsByOwnerByRepoByRef,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.Contains(req.RequestURI, "test-error-response") {
					mock.WriteError(w,
						http.StatusInternalServerError,
						"github went belly up or something",
					)
				} else {
					/* #nosec G104 -- test code */
					w.Write([]byte("ca82a6dff817ec66f44342007202690a93763949"))
				}
			}),
		),
		mock.WithRequestMatchHandler(
			mock.GetReposByOwnerByRepo,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.Contains(req.RequestURI, "test-error-response") {
					mock.WriteError(w,
						http.StatusInternalServerError,
						"github went belly up or something",
					)
				} else if strings.Contains(req.RequestURI, "test-repo-2") {
					/* #nosec G104 -- test code */
					w.Write(mock.MustMarshal(github.Repository{
						Name:          github.String("test-repo-2"),
						DefaultBranch: github.String("master"),
					}))
				} else {
					/* #nosec G104 -- test code */
					w.Write(mock.MustMarshal(github.Repository{
						Name:          github.String("test-repo-1"),
						DefaultBranch: github.String("main"),
					}))
				}
			}),
		),
		mock.WithRequestMatchHandler(
			mock.GetReposBranchesByOwnerByRepoByBranch,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.Contains(req.RequestURI, "test-repo-2") && strings.Contains(req.RequestURI, "master") {
					/* #nosec G104 -- test code */
					w.Write(mock.MustMarshal(github.Branch{
						Name: github.String("master"),
					}))
				} else if strings.Contains(req.RequestURI, "test-repo-1") && strings.Contains(req.RequestURI, "main") {
					/* #nosec G104 -- test code */
					w.Write(mock.MustMarshal(github.Branch{
						Name: github.String("main"),
					}))
				} else {
					mock.WriteError(w,
						http.StatusInternalServerError,
						"github went belly up or something",
					)
				}
			}),
		),
	)

	cl, _ := createGitHubClientFromToken(&mockedHTTPClient.Transport, "", "mock")
	return cl.Client

}

func GetMockedPrimaryRateLimitedClient() *github.Client {
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetRateLimit,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				/* #nosec G104 -- test code */
				response := new(struct {
					Resources *github.RateLimits `json:"resources"`
				})
				response.Resources = &github.RateLimits{
					Core: &github.Rate{
						Limit:     5000,
						Remaining: 0,
					},
					Search: &github.Rate{
						Limit:     30,
						Remaining: 0,
					},
				}
				w.Write(mock.MustMarshal(response))
			}),
		),
	)

	cl, _ := createGitHubClientFromToken(&mockedHTTPClient.Transport, "", "mock")
	return cl.Client

}

func GetMockedResetPrimaryRateLimitedClient() *github.Client {
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetRateLimit,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				/* #nosec G104 -- test code */
				response := new(struct {
					Resources *github.RateLimits `json:"resources"`
				})
				response.Resources = &github.RateLimits{
					Core: &github.Rate{
						Limit:     5000,
						Remaining: 4999,
					},
					Search: &github.Rate{
						Limit:     30,
						Remaining: 29,
					},
				}
				w.Write(mock.MustMarshal(response))
			}),
		),
	)

	cl, _ := createGitHubClientFromToken(&mockedHTTPClient.Transport, "", "mock")
	return cl.Client
}

func GetMockedSecondaryRateLimitedClient() *github.Client {
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetRateLimit,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				/* #nosec G104 -- test code */
				response := new(struct {
					Resources *github.RateLimits `json:"resources"`
				})
				response.Resources = &github.RateLimits{
					Core: &github.Rate{
						Limit:     5000,
						Remaining: 100,
					},
					Search: &github.Rate{
						Limit:     30,
						Remaining: 15,
					},
				}
				w.Write(mock.MustMarshal(response))
			}),
		),
		mock.WithRequestMatchHandler(
			mock.PostOrgsReposByOrg,
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Add("Retry-After", "3")
				WriteError(w,
					http.StatusForbidden,
					"secondary rate limit",
				)
			}),
		),
	)

	cl, _ := createGitHubClientFromToken(&mockedHTTPClient.Transport, "", "mock")
	return cl.Client

}

// WriteError - based on the mock implementation to handle writing back a response
// workaround until PR https://github.com/migueleliasweb/go-github-mock/pull/41 is merged
func WriteError(
	w http.ResponseWriter,
	httpStatus int,
	msg string,
	errors ...github.Error,
) {
	w.WriteHeader(httpStatus)

	w.Write(mock.MustMarshal(mockGitHubErrorResponse{
		Message: msg,
		Errors:  errors,
	}))

}

type mockGitHubErrorResponse struct {
	Message string         `json:"message"` // error message
	Errors  []github.Error `json:"errors"`  // more detail on individual errors
}

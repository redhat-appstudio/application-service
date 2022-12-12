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
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/v41/github"
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
				if strings.Contains(reqBody, "test-error-response") {
					mock.WriteError(w,
						http.StatusInternalServerError,
						"github went belly up or something",
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
	)

	return github.NewClient(mockedHTTPClient)

}

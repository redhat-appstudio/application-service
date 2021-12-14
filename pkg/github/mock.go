package github

import (
	"net/http"

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
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write(mock.MustMarshal(github.Repository{
					Name: github.String("test-repo-1"),
				}))
			}),
		),
		mock.WithRequestMatchHandler(
			mock.DeleteReposByOwnerByRepo,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write(mock.MustMarshal(github.Repository{
					Name: github.String("test-repo-1"),
				}))
			}),
		),
	)

	return github.NewClient(mockedHTTPClient)

}

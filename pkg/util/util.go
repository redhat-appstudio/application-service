//
// Copyright 2021-2022 Red Hat, Inc.
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

package util

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-git/go-git/v5"
	transportHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
)

func SanitizeName(name string) string {
	sanitizedName := strings.ToLower(strings.Replace(strings.Replace(name, " ", "-", -1), "'", "", -1))
	if len(sanitizedName) > 50 {
		sanitizedName = sanitizedName[0:50]
	}

	return sanitizedName
}

// IsExist returns whether the given file or directory exists
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ProcessGitOpsStatus processes the GitOps status and returns the remote url, branch, context and the error
func ProcessGitOpsStatus(gitopsStatus appstudiov1alpha1.GitOpsStatus, gitToken string) (string, string, string, error) {
	var gitOpsURL, gitOpsBranch, gitOpsContext string
	gitOpsURL = gitopsStatus.RepositoryURL
	if gitOpsURL == "" {
		err := fmt.Errorf("unable to process GitOps status, GitOps Repository URL cannot be empty")
		return "", "", "", err
	}
	if gitopsStatus.Branch != "" {
		gitOpsBranch = gitopsStatus.Branch
	} else {
		gitOpsBranch = "main"
	}
	if gitopsStatus.Context != "" {
		gitOpsContext = gitopsStatus.Context
	} else {
		gitOpsContext = "/"
	}

	// Construct the remote URL for the gitops repository
	parsedURL, err := url.Parse(gitOpsURL)
	if err != nil {
		return "", "", "", err
	}
	parsedURL.User = url.User(gitToken)
	remoteURL := parsedURL.String()

	return remoteURL, gitOpsBranch, gitOpsContext, nil
}

// ConvertGitHubURL converts a git url to its raw format
// adapted from https://github.com/redhat-developer/odo/blob/e63773cc156ade6174a533535cbaa0c79506ffdb/pkg/catalog/catalog.go#L72
func ConvertGitHubURL(URL string, revision string) (string, error) {
	// If the URL ends with .git, remove it
	// The regex will only instances of '.git' if it is at the end of the given string
	reg := regexp.MustCompile(".git$")
	URL = reg.ReplaceAllString(URL, "")

	// If the URL has a trailing / suffix, trim it
	URL = strings.TrimSuffix(URL, "/")

	url, err := url.Parse(URL)
	if err != nil {
		return "", err
	}

	if strings.Contains(url.Host, "github") && !strings.Contains(url.Host, "raw") {
		// Convert path part of the URL
		URLSlice := strings.Split(URL, "/")
		if len(URLSlice) > 2 && URLSlice[len(URLSlice)-2] == "tree" {
			// GitHub raw URL doesn't have "tree" structure in the URL, need to remove it
			URL = strings.Replace(URL, "/tree", "", 1)
		} else if revision != "" {
			// Add revision for GitHub raw URL
			URL = URL + "/" + revision
		} else {
			// Add "main" branch for GitHub raw URL by default if revision is not specified
			URL = URL + "/main"
		}

		// Convert host part of the URL
		if url.Host == "github.com" {
			URL = strings.Replace(URL, "github.com", "raw.githubusercontent.com", 1)
		}
	}

	return URL, nil
}

// CurlEndpoint curls the endpoint and returns the response or an error if the response is a non-200 status
func CurlEndpoint(endpoint string) ([]byte, error) {
	var respBytes []byte
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		respBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return respBytes, nil
	}

	return nil, fmt.Errorf("received a non-200 status when curling %s", endpoint)
}

// CloneRepo clones the repoURL to clonePath
func CloneRepo(clonePath, repoURL string, token string) error {
	// Set up the Clone options
	cloneOpts := &git.CloneOptions{
		URL: repoURL,
	}

	// If a token was passed in, configure token auth for the git client
	if token != "" {
		cloneOpts.Auth = &transportHttp.BasicAuth{
			Username: "token",
			Password: token,
		}
	}
	// Clone the repo
	_, err := git.PlainClone(clonePath, false, cloneOpts)
	if err != nil {
		return err
	}

	return nil
}

// SanitizeErrorMessage takes in a given error message and returns a new, santized error with things like tokens, removed
func SanitizeErrorMessage(err error) error {
	reg := regexp.MustCompile(`ghp_[a-zA-Z0-9]*`)
	newErrMsg := reg.ReplaceAllString(err.Error(), "<TOKEN>")
	return fmt.Errorf(newErrMsg)
}

// CheckWithRegex checks if a name matches the pattern.
// If a pattern fails to compile, it returns false
func CheckWithRegex(pattern, name string) bool {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return reg.MatchString(name)
}

func GetMappedGitOpsComponent(component appstudiov1alpha1.Component) gitopsgenv1alpha1.Component {
	gitopsMapComponent := gitopsgenv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.ObjectMeta.Name,
			Namespace: component.ObjectMeta.Namespace,
		},
		Spec: gitopsgenv1alpha1.ComponentSpec{
			ComponentName:                component.Spec.ComponentName,
			Application:                  component.Spec.Application,
			Secret:                       component.Spec.Secret,
			Resources:                    component.Spec.Resources,
			Replicas:                     component.Spec.Replicas,
			TargetPort:                   component.Spec.TargetPort,
			Route:                        component.Spec.Route,
			Env:                          component.Spec.Env,
			ContainerImage:               component.Spec.ContainerImage,
			SkipGitOpsResourceGeneration: component.Spec.SkipGitOpsResourceGeneration,
		},
	}
	if component.Spec.Source.ComponentSourceUnion.GitSource != nil {
		gitopsMapComponent.Spec.Source = gitopsgenv1alpha1.ComponentSource{
			ComponentSourceUnion: gitopsgenv1alpha1.ComponentSourceUnion{
				GitSource: &gitopsgenv1alpha1.GitSource{
					URL:           component.Spec.Source.ComponentSourceUnion.GitSource.URL,
					Revision:      component.Spec.Source.ComponentSourceUnion.GitSource.Revision,
					Context:       component.Spec.Source.ComponentSourceUnion.GitSource.Context,
					DevfileURL:    component.Spec.Source.ComponentSourceUnion.GitSource.DevfileURL,
					DockerfileURL: component.Spec.Source.ComponentSourceUnion.GitSource.DockerfileURL,
				},
			},
		}
	}
	return gitopsMapComponent
}

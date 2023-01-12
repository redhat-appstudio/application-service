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

package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devfile/registry-support/index/generator/schema"
	registryLibrary "github.com/devfile/registry-support/registry-library/library"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

// CloneRepo clones the repoURL to clonePath
func CloneRepo(clonePath, repoURL string, token string) error {
	exist, err := IsExist(clonePath)
	if !exist || err != nil {
		os.MkdirAll(clonePath, 0755)
	}
	cloneURL := repoURL
	// Execute does an exec.Command on the specified command
	if token != "" {
		tempStr := strings.Split(repoURL, "https://")

		// e.g. https://token:<token>@github.com/owner/repoName.git
		cloneURL = fmt.Sprintf("https://token:%s@%s", token, tempStr[1])
	}
	c := exec.Command("git", "clone", cloneURL, clonePath)
	c.Dir = clonePath

	// set env to skip authentication prompt and directly error out
	c.Env = os.Environ()
	c.Env = append(c.Env, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/echo")

	_, err = c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone the repo: %v", err)
	}

	return nil
}

// CurlEndpoint curls the endpoint and returns the response or an error if the response is a non-200 status
func CurlEndpoint(endpoint string) ([]byte, error) {
	var respBytes []byte
	/* #nosec G107 --  The URL is validated by the CDQ if the request is coming from the UI.  If we do happen to download invalid bytes, the devfile parser will catch this and fail. */
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

// ConvertGitHubURL converts a git url to its raw format
// adapted from https://github.com/redhat-developer/odo/blob/e63773cc156ade6174a533535cbaa0c79506ffdb/pkg/catalog/catalog.go#L72
func ConvertGitHubURL(URL string, revision string, context string) (string, error) {
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
		if context != "" && context != "./" && context != "." {
			// trim the prefix / in context
			context = strings.TrimPrefix(context, "/")
			URL = URL + "/" + context
		}

		// Convert host part of the URL
		if url.Host == "github.com" {
			URL = strings.Replace(URL, "github.com", "raw.githubusercontent.com", 1)
		}
	}

	return URL, nil
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

// getAlizerDevfileTypes gets the Alizer devfile types for a specified registry
func getAlizerDevfileTypes(registryURL string) ([]recognizer.DevFileType, error) {
	types := []recognizer.DevFileType{}
	registryIndex, err := registryLibrary.GetRegistryIndex(registryURL, registryLibrary.RegistryOptions{
		Telemetry: registryLibrary.TelemetryData{},
	}, schema.SampleDevfileType)
	if err != nil {
		return nil, err
	}

	for _, index := range registryIndex {
		types = append(types, recognizer.DevFileType{
			Name:        index.Name,
			Language:    index.Language,
			ProjectType: index.ProjectType,
			Tags:        index.Tags,
		})
	}

	return types, nil
}

// GetRepoFromRegistry gets the sample repo link from the devfile registry
func GetRepoFromRegistry(name, registryURL string) (string, error) {
	registryIndex, err := registryLibrary.GetRegistryIndex(registryURL, registryLibrary.RegistryOptions{
		Telemetry: registryLibrary.TelemetryData{},
	}, schema.SampleDevfileType)
	if err != nil {
		return "", err
	}

	for _, index := range registryIndex {
		if index.Name == name && index.Git != nil && index.Git.Remotes["origin"] != "" {
			return index.Git.Remotes["origin"], nil
		}
	}

	return "", fmt.Errorf("unable to find sample with a name %s in the registry", name)
}

// getContext returns the context backtracking from the end of the localpath
func getContext(localpath string, currentLevel int) string {
	context := "./"
	currentPath := localpath
	for i := 0; i < currentLevel; i++ {
		context = path.Join(filepath.Base(currentPath), context)
		currentPath = filepath.Dir(currentPath)
	}

	return context
}

// UpdateGitLink updates the relative uri
// to a full URL link with the context & revision
func UpdateGitLink(repo, revision, context string) (string, error) {
	var rawGitURL string
	var err error
	if !strings.HasPrefix(context, "http") {
		rawGitURL, err = ConvertGitHubURL(repo, revision, context)
		if err != nil {
			return "", err
		}

	} else {
		return context, nil
	}

	return rawGitURL, nil
}

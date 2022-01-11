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

package util

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

const (
	devfileName       = "devfile.yaml"
	hiddenDevfileName = ".devfile.yaml"
)

func SanitizeDisplayName(displayName string) string {
	sanitizedName := strings.ToLower(strings.Replace(displayName, " ", "-", -1))
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

// ConvertGitHubURL converts a git url to its raw format
// taken from Jingfu's odo code
func ConvertGitHubURL(URL string) (string, error) {
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
		} else {
			// Add "main" branch for GitHub raw URL by default if branch is not specified
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
func CloneRepo(clonePath, repoURL string) error {
	// Check if the clone path is empty, if not delete it
	isDirExist, err := IsExist(clonePath)
	if err != nil {
		return err
	}
	fmt.Printf("isDirExist %v\n", isDirExist)
	if isDirExist {
		fmt.Printf("removing %v\n", isDirExist)
		os.RemoveAll(clonePath)
	}

	// Clone the repo
	_, err = git.PlainClone(clonePath, false, &git.CloneOptions{
		URL: repoURL,
	})
	if err != nil {
		return err
	}

	return nil
}

func ReadDevfilesFromRepo(localpath string, depth int) (map[string][]byte, error) {
	return searchDevfiles(localpath, 0, depth)
}

func searchDevfiles(localpath string, currentLevel, depth int) (map[string][]byte, error) {

	var devfileBytes []byte
	devfileMapFromRepo := make(map[string][]byte)

	files, err := ioutil.ReadDir(localpath)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if (f.Name() == devfileName || f.Name() == hiddenDevfileName) && currentLevel != 0 {
			devfileBytes, err = ioutil.ReadFile(path.Join(localpath, f.Name()))
			if err != nil {
				return nil, err
			}

			var context string
			currentPath := localpath
			for i := 0; i < currentLevel; i++ {
				context = path.Join(filepath.Base(currentPath), context)
				currentPath = filepath.Dir(currentPath)
			}
			devfileMapFromRepo[context] = devfileBytes
		} else if f.IsDir() {
			if currentLevel+1 <= depth {
				recursiveMap, err := searchDevfiles(path.Join(localpath, f.Name()), currentLevel+1, depth)
				if err != nil {
					return nil, err
				}
				for context, devfile := range recursiveMap {
					devfileMapFromRepo[context] = devfile
				}
			}
		}
	}

	return devfileMapFromRepo, nil
}

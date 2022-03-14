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
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	transportHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/redhat-appstudio/application-service/pkg/devfile"

	"github.com/devfile/registry-support/index/generator/schema"
	registryLibrary "github.com/devfile/registry-support/registry-library/library"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

const (

	// DevfileStageRegistryEndpoint is the endpoint of the staging devfile registry
	DevfileStageRegistryEndpoint = "https://registry.stage.devfile.io"
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

// ConvertGitHubURL converts a git url to its raw format
// adapted from https://github.com/redhat-developer/odo/blob/e63773cc156ade6174a533535cbaa0c79506ffdb/pkg/catalog/catalog.go#L72
func ConvertGitHubURL(URL string) (string, error) {
	// If the URL ends with .git, remove it
	// The regex will only instances of '.git' if it is at the end of the given string
	reg := regexp.MustCompile(".git$")
	URL = reg.ReplaceAllString(URL, "")

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

// DownloadDevfile downloads devfile from the various possible devfile locations in dir and returns the contents
func DownloadDevfile(dir string) ([]byte, error) {
	var devfileBytes []byte
	var err error
	validDevfileLocations := []string{devfile.Devfile, devfile.HiddenDevfile, devfile.HiddenDirDevfile, devfile.HiddenDirHiddenDevfile}

	for _, path := range validDevfileLocations {
		devfilePath := dir + "/" + path
		devfileBytes, err = CurlEndpoint(devfilePath)
		if err == nil {
			// if we get a 200, return
			return devfileBytes, err
		}
	}

	return nil, &NoDevfileFound{location: dir}
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
	// Check if the clone path is empty, if not delete it
	isDirExist, err := IsExist(clonePath)
	if err != nil {
		return err
	}

	if isDirExist {
		os.RemoveAll(clonePath)
	}

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
	_, err = git.PlainClone(clonePath, false, cloneOpts)
	if err != nil {
		return err
	}

	return nil
}

// ReadDevfilesFromRepo attempts to read and return devfiles from the local path upto the specified depth
func ReadDevfilesFromRepo(localpath string, depth int) (map[string][]byte, map[string]string, error) {
	return searchDevfiles(localpath, 0, depth)
}

func searchDevfiles(localpath string, currentLevel, depth int) (map[string][]byte, map[string]string, error) {

	devfileMapFromRepo := make(map[string][]byte)
	devfilesURLMapFromRepo := make(map[string]string)

	isDevfilePresent := false

	files, err := ioutil.ReadDir(localpath)
	if err != nil {
		return nil, nil, err
	}

	for _, f := range files {
		if (f.Name() == devfile.DevfileName || f.Name() == devfile.HiddenDevfileName) && currentLevel != 0 {
			devfileBytes, err := ioutil.ReadFile(path.Join(localpath, f.Name()))
			if err != nil {
				return nil, nil, err
			}

			context := getContext(localpath, currentLevel)
			devfileMapFromRepo[context] = devfileBytes
			isDevfilePresent = true
		} else if f.IsDir() && f.Name() == devfile.HiddenDevfileDir {
			// if the dir is .devfile, we dont increment currentLevel
			// consider devfile.yaml and .devfile/devfile.yaml as the same level
			recursiveDevfileMap, recursiveDevfileURLMap, err := searchDevfiles(path.Join(localpath, f.Name()), currentLevel, depth)
			if err != nil {
				return nil, nil, err
			}

			context := getContext(localpath, currentLevel)
			for recursiveContext := range recursiveDevfileMap {
				if recursiveContext == devfile.HiddenDevfileDir {
					devfileMapFromRepo[context] = recursiveDevfileMap[devfile.HiddenDevfileDir]
					devfilesURLMapFromRepo[context] = recursiveDevfileURLMap[devfile.HiddenDevfileDir]
					isDevfilePresent = true
				}
			}
		} else if f.IsDir() {
			if currentLevel+1 <= depth {
				recursiveDevfileMap, recursiveDevfileURLMap, err := searchDevfiles(path.Join(localpath, f.Name()), currentLevel+1, depth)
				if err != nil {
					return nil, nil, err
				}
				for context, devfile := range recursiveDevfileMap {
					devfileMapFromRepo[context] = devfile
					devfilesURLMapFromRepo[context] = recursiveDevfileURLMap[context]
					isDevfilePresent = true
				}
			}
		}
	}

	if len(devfileMapFromRepo) == 0 && currentLevel == 0 {
		// if we didnt find any devfile we should return an err
		err = &NoDevfileFound{location: localpath}
	} else if !isDevfilePresent && currentLevel == depth {
		// if we didnt find any devfile upto our desired depth, then use alizer
		devfileBytes, detectedDevfileEndpoint, err := AnalyzeAndDetectDevfile(localpath)
		if err != nil {
			if _, ok := err.(*NoDevfileFound); !ok {
				return nil, nil, err
			}
		}

		if len(devfileBytes) > 0 {
			context := getContext(localpath, currentLevel)
			devfileMapFromRepo[context] = devfileBytes
			devfilesURLMapFromRepo[context] = detectedDevfileEndpoint
		}

	}

	return devfileMapFromRepo, devfilesURLMapFromRepo, err
}

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

// AnalyzeAndDetectDevfile analyzes and attempts to detect a devfile from the devfile registry for a given local path
func AnalyzeAndDetectDevfile(path string) ([]byte, string, error) {
	var devfileBytes []byte

	alizerLanguages, err := recognizer.Analyze(path)
	if err != nil {
		return nil, "", err
	}

	alizerTypes, err := getAlizerDevfileTypes(DevfileStageRegistryEndpoint)
	if err != nil {
		return nil, "", err
	}

	for _, language := range alizerLanguages {
		if language.CanBeComponent {
			// if we get one language analysis that can be a component
			// we can then determine a devfile from the registry and return

			detectedType, err := recognizer.SelectDevFileFromTypes(path, alizerTypes)
			if err != nil && err.Error() != fmt.Sprintf("No valid devfile found for project in %s", path) {
				// No need to check for err, if a path does not have a detected devfile, ignore err
				// if a dir can be a component but we get an unrelated err, err out
				return nil, "", err
			} else if !reflect.DeepEqual(detectedType, recognizer.DevFileType{}) {
				detectedDevfileEndpoint := DevfileStageRegistryEndpoint + "/devfiles/" + detectedType.Name

				devfileBytes, err = CurlEndpoint(detectedDevfileEndpoint)
				if err != nil {
					return nil, "", err
				}

				if len(devfileBytes) > 0 {
					return devfileBytes, detectedDevfileEndpoint, nil
				}
			}
		}
	}

	return nil, "", &NoDevfileFound{location: path}
}

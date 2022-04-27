//
// Copyright 2022 Red Hat, Inc.
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

package devfile

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/devfile/registry-support/index/generator/schema"
	registryLibrary "github.com/devfile/registry-support/registry-library/library"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

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

func UpdateDockerfileLink(repo, context string) (string, error) {

	link := context

	if !strings.HasPrefix(context, "http") {
		rawGitURL, err := util.ConvertGitHubURL(repo)
		if err != nil {
			return "", err
		}

		if !strings.HasSuffix(rawGitURL, "/") {
			rawGitURL = rawGitURL + "/"
		}

		link = rawGitURL + link
	}

	return link, nil
}

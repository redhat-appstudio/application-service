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
	"path"
	"path/filepath"

	"github.com/devfile/registry-support/index/generator/schema"
	registryLibrary "github.com/devfile/registry-support/registry-library/library"
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

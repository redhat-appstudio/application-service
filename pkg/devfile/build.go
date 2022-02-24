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
	"errors"
	"os"
	"path"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/spf13/afero"
)

const (
	DevfileBuild = "devfile-build"
	DockerBuild  = "docker-build"
	JavaBuild    = "java-builder"
	NodeJsBuild  = "nodejs-builder"
	NoOpBuild    = "noop"

	DockerfileName      = "Dockerfile"
	PackageJsonFileName = "package.json"
)

func DetermineBuildPipeline(appFs afero.Fs, rootDir string, component *appstudiov1alpha1.Component) (string, error) {
	// First try to find devfiles
	devfiles, err := ReadDevfilesFromRepo(rootDir, 1)
	if err != nil {
		return "", err
	}
	if len(devfiles) > 0 {
		return DevfileBuild, nil
	}

	// Devfile is not present, try to find a Dockerfile
	if dockerfileExists, err := fileExists(appFs, path.Join(rootDir, DockerfileName)); dockerfileExists || err != nil {
		if err != nil {
			return "", err
		}
		if dockerfileExists {
			return DockerBuild, nil
		}
	}

	// Check if nodejs is used
	if packageJsonExists, err := fileExists(appFs, path.Join(rootDir, PackageJsonFileName)); packageJsonExists || err != nil {
		if err != nil {
			return "", err
		}
		if packageJsonExists {
			return NodeJsBuild, nil
		}
	}

	// TODO try to detect other build types

	// Nothing above worked, return noop build
	return NoOpBuild, nil
}

func fileExists(appFs afero.Fs, pathToFile string) (bool, error) {
	_, err := appFs.Stat(pathToFile)
	if err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}

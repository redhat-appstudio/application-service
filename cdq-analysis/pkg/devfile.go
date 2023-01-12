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

package pkg

import (
	devfilePkg "github.com/devfile/library/pkg/devfile"
	parser "github.com/devfile/library/pkg/devfile/parser"
	data "github.com/devfile/library/pkg/devfile/parser/data"

	"github.com/go-logr/logr"
)

const (
	DevfileName       = "devfile.yaml"
	HiddenDevfileName = ".devfile.yaml"
	HiddenDevfileDir  = ".devfile"
	DockerfileName    = "Dockerfile"

	Devfile                = DevfileName                                // devfile.yaml
	HiddenDevfile          = HiddenDevfileName                          // .devfile.yaml
	HiddenDirDevfile       = HiddenDevfileDir + "/" + DevfileName       // .devfile/devfile.yaml
	HiddenDirHiddenDevfile = HiddenDevfileDir + "/" + HiddenDevfileName // .devfile/.devfile.yaml

	// DevfileRegistryEndpoint is the endpoint of the devfile registry
	DevfileRegistryEndpoint = "https://registry.devfile.io"

	// DevfileStageRegistryEndpoint is the endpoint of the staging devfile registry
	DevfileStageRegistryEndpoint = "https://registry.stage.devfile.io"
)

// ParseDevfileModel calls the devfile library's parse and returns the devfile data
func ParseDevfileModel(devfileModel string) (data.DevfileData, error) {
	// Retrieve the devfile from the body of the resource
	devfileBytes := []byte(devfileModel)
	parserArgs := parser.ParserArgs{
		Data: devfileBytes,
	}
	devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parserArgs)
	return devfileObj.Data, err
}

// FindAndDownloadDevfile downloads devfile from the various possible devfile locations in dir and returns the contents and its context
func FindAndDownloadDevfile(dir string) ([]byte, string, error) {
	var devfileBytes []byte
	var err error
	validDevfileLocations := []string{Devfile, HiddenDevfile, HiddenDirDevfile, HiddenDirHiddenDevfile}

	for _, path := range validDevfileLocations {
		devfilePath := dir + "/" + path
		devfileBytes, err = DownloadFile(devfilePath)
		if err == nil {
			// if we get a 200, return
			return devfileBytes, path, err
		}
	}

	return nil, "", &NoDevfileFound{Location: dir}
}

// DownloadFile downloads the specified file
func DownloadFile(file string) ([]byte, error) {
	return CurlEndpoint(file)
}

// DownloadDevfileAndDockerfile attempts to download and return the devfile, devfile context and dockerfile from the root of the specified url
func DownloadDevfileAndDockerfile(url string) ([]byte, string, []byte) {
	var devfileBytes, dockerfileBytes []byte
	var devfilePath string

	devfileBytes, devfilePath, _ = FindAndDownloadDevfile(url)
	dockerfileBytes, _ = DownloadFile(url + "/Dockerfile")

	return devfileBytes, devfilePath, dockerfileBytes
}

// ScanRepo attempts to read and return devfiles and dockerfiles from the local path upto the specified depth
// Iterate through each sub-folder under first level, and scan for component. (devfile, dockerfile, then Alizer)
// If no devfile(s) or dockerfile(s) are found in sub-folders of the root directory, then the Alizer tool is used to detect and match a devfile/dockerfile from the devfile registry
// ScanRepo returns 3 maps and an error:
// Map 1 returns a context to the devfile bytes if present.
// Map 2 returns a context to the matched devfileURL from the devfile registry if no devfile is present in the context.
// Map 3 returns a context to the dockerfile uri or a matched dockerfileURL from the devfile registry if no dockerfile is present in the context
func ScanRepo(log logr.Logger, a Alizer, localpath string, devfileRegistryURL string) (map[string][]byte, map[string]string, map[string]string, error) {
	return search(log, a, localpath, devfileRegistryURL)
}

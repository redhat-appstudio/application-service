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

	devfilePkg "github.com/devfile/library/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/devfile/library/v2/pkg/devfile/parser/data"

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

// DevfileSrc specifies the src of the Devfile
type DevfileSrc struct {
	Data string
	URL  string
}

// ParseDevfile calls the devfile library's parse and returns the devfile data.
// Provide either a Data src or the URL src
func ParseDevfile(src DevfileSrc) (data.DevfileData, error) {

	httpTimeout := 10
	convert := true
	parserArgs := parser.ParserArgs{
		HTTPTimeout:                   &httpTimeout,
		ConvertKubernetesContentInUri: &convert,
	}

	if src.Data != "" {
		parserArgs.Data = []byte(src.Data)
	} else if src.URL != "" {
		parserArgs.URL = src.URL
	} else {
		return nil, fmt.Errorf("cannot parse devfile without a src")
	}

	devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parserArgs)
	return devfileObj.Data, err
}

// ScanRepo attempts to read and return devfiles and dockerfiles from the local path upto the specified depth
// Iterate through each sub-folder under first level, and scan for component. (devfile, dockerfile, then Alizer)
// If no devfile(s) or dockerfile(s) are found in sub-folders of the root directory, then the Alizer tool is used to detect and match a devfile/dockerfile from the devfile registry
// ScanRepo returns 3 maps and an error:
// Map 1 returns a context to the devfile bytes if present.
// Map 2 returns a context to the matched devfileURL from the devfile registry if no devfile is present in the context.
// Map 3 returns a context to the dockerfile uri or a matched dockerfileURL from the devfile registry if no dockerfile is present in the context
func ScanRepo(log logr.Logger, a Alizer, localpath string, devfileRegistryURL string, URL, revision, srcContext string) (map[string][]byte, map[string]string, map[string]string, error) {
	return search(log, a, localpath, devfileRegistryURL, URL, revision, srcContext)
}

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
	"net/url"
	"path"
	"strings"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfileValidation "github.com/devfile/api/v2/pkg/validation"
	devfilePkg "github.com/devfile/library/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	parserUtil "github.com/devfile/library/v2/pkg/util"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/yaml"
)

const (
	DevfileName             = "devfile.yaml"
	HiddenDevfileName       = ".devfile.yaml"
	HiddenDevfileDir        = ".devfile"
	DockerfileName          = "Dockerfile"
	AlternateDockerfileName = "dockerfile"
	ContainerfileName       = "Containerfile"
	HiddenDockerDir         = ".docker"
	DockerDir               = "docker"
	BuildDir                = "build"

	Devfile                = DevfileName                                // devfile.yaml
	HiddenDevfile          = HiddenDevfileName                          // .devfile.yaml
	HiddenDirDevfile       = HiddenDevfileDir + "/" + DevfileName       // .devfile/devfile.yaml
	HiddenDirHiddenDevfile = HiddenDevfileDir + "/" + HiddenDevfileName // .devfile/.devfile.yaml

	Dockerfile                   = DockerfileName                                  // Dockerfile
	HiddenDirDockerfile          = HiddenDockerDir + "/" + DockerfileName          // .docker/Dockerfile
	DockerDirDockerfile          = DockerDir + "/" + DockerfileName                // docker/Dockerfile
	BuildDirDockerfile           = BuildDir + "/" + DockerfileName                 // build/Dockerfile
	AlternateDockerfile          = AlternateDockerfileName                         // dockerfile
	HiddenDirAlternateDockerfile = HiddenDockerDir + "/" + AlternateDockerfileName // .docker/dockerfile
	DockerDirAlternateDockerfile = DockerDir + "/" + AlternateDockerfileName       // docker/dockerfile
	BuildDirAlternateDockerfile  = BuildDir + "/" + AlternateDockerfileName        // build/dockerfile

	Containerfile          = ContainerfileName                         // Containerfile
	HiddenDirContainerfile = HiddenDockerDir + "/" + ContainerfileName // .docker/Containerfile
	DockerDirContainerfile = DockerDir + "/" + ContainerfileName       // docker/Containerfile
	BuildDirContainerfile  = BuildDir + "/" + ContainerfileName        // build/Containerfile

	// DevfileRegistryEndpoint is the endpoint of the devfile registry
	DevfileRegistryEndpoint = "https://registry.devfile.io"

	// DevfileStageRegistryEndpoint is the endpoint of the staging devfile registry
	DevfileStageRegistryEndpoint = "https://registry.stage.devfile.io"
)

// ScanRepo attempts to read and return devfiles and dockerfiles from the local path upto the specified depth
// Iterate through each sub-folder under first level, and scan for component. (devfile, dockerfile, then Alizer)
// If no devfile(s) or dockerfile(s) are found in sub-folders of the root directory, then the Alizer tool is used to detect and match a devfile/dockerfile from the devfile registry
// ScanRepo returns 3 maps and an error:
// Map 1 returns a context to the devfile bytes if present.
// Map 2 returns a context to the matched devfileURL from the devfile registry if no devfile is present in the context.
// Map 3 returns a context to the Dockerfile uri or a matched DockerfileURL from the devfile registry if no Dockerfile/Containerfile is present in the context
// Map 4 returns a context to the list of ports that were detected by alizer in the source code, at that given context
func ScanRepo(log logr.Logger, a Alizer, localpath string, devfileRegistryURL string, URL, revision, srcContext string) (map[string][]byte, map[string]string, map[string]string, map[string][]int, error) {
	return search(log, a, localpath, devfileRegistryURL, URL, revision, srcContext)
}

// ValidateDevfile parse and validate a devfile from it's URL, returns if the devfile should be ignored, the devfile raw content and an error if devfile is invalid
// If the devfile failed to parse, or the kubernetes uri is invalid or kubernetes file content is invalid. return an error.
// If no kubernetes components being defined in devfile, then it's not a valid outerloop devfile, the devfile should be ignored.
// If more than one kubernetes components in the devfile, but no deploy commands being defined. return an error
// If more than one image components in the devfile, but no apply commands being defined. return an error
func ValidateDevfile(log logr.Logger, URL string) (shouldIgnoreDevfile bool, devfileBytes []byte, err error) {
	log.Info(fmt.Sprintf("Validating devfile from %s...", URL))
	shouldIgnoreDevfile = false
	var devfileSrc DevfileSrc
	if strings.HasPrefix(URL, "http://") || strings.HasPrefix(URL, "https://") {
		devfileSrc = DevfileSrc{
			URL: URL,
		}
	} else {
		devfileSrc = DevfileSrc{
			Path: URL,
		}
	}

	devfileData, err := ParseDevfile(devfileSrc)
	if err != nil {
		var newErr error
		if merr, ok := err.(*multierror.Error); ok {
			for i := range merr.Errors {
				switch merr.Errors[i].(type) {
				case *devfileValidation.MissingDefaultCmdWarning:
					log.Info(fmt.Sprintf("devfile is missing default command, found a warning: %v", merr.Errors[i]))
				default:
					newErr = multierror.Append(newErr, merr.Errors[i])
				}
			}
		} else {
			newErr = err
		}
		if newErr != nil {
			if merr, ok := newErr.(*multierror.Error); !ok || len(merr.Errors) != 0 {
				log.Error(err, fmt.Sprintf("failed to parse the devfile content from %s", URL))
				return shouldIgnoreDevfile, nil, fmt.Errorf(fmt.Sprintf("err: %v, failed to parse the devfile content from %s", err, URL))
			}
		}
	}
	deployCompMap, err := parser.GetDeployComponents(devfileData)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get deploy components from %s", URL))
		return shouldIgnoreDevfile, nil, fmt.Errorf(fmt.Sprintf("err: %v, failed to get deploy components from %s", err, URL))
	}
	devfileBytes, err = yaml.Marshal(devfileData)
	if err != nil {
		return shouldIgnoreDevfile, nil, err
	}
	kubeCompFilter := common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.KubernetesComponentType,
		},
	}
	kubeComp, err := devfileData.GetComponents(kubeCompFilter)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get kubernetes component from %s", URL))
		shouldIgnoreDevfile = true
		return shouldIgnoreDevfile, nil, nil
	}
	if len(kubeComp) == 0 {
		log.Info(fmt.Sprintf("Found 0 kubernetes components being defined in devfile from %s, it is not a valid outerloop definition, the devfile will be ignored. A devfile will be matched from registry...", URL))
		shouldIgnoreDevfile = true
		return shouldIgnoreDevfile, nil, nil
	} else {
		if len(kubeComp) > 1 {
			found := false
			for _, component := range kubeComp {
				if _, ok := deployCompMap[component.Name]; ok {
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("found more than one kubernetes components, but no deploy command associated with any being defined in the devfile from %s", URL)
				log.Error(err, "failed to validate devfile")
				return shouldIgnoreDevfile, nil, err
			}
		}
		// TODO: if only one kube component, should return a warning that no deploy command being defined
	}
	imageCompFilter := common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.ImageComponentType,
		},
	}
	imageComp, err := devfileData.GetComponents(imageCompFilter)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get image component from %s", URL))
		return shouldIgnoreDevfile, nil, fmt.Errorf(fmt.Sprintf("err: %v, failed to get image component from %s", err, URL))
	}
	if len(imageComp) == 0 {
		log.Info(fmt.Sprintf("Found 0 image components being defined in devfile from %s, it is not a valid outerloop definition, the devfile will be ignored. A devfile will be matched from registry...", URL))
		shouldIgnoreDevfile = true
		return shouldIgnoreDevfile, nil, nil
	} else {
		if len(imageComp) > 1 {
			found := false
			for _, component := range imageComp {
				if component.Image != nil && component.Image.Dockerfile != nil && component.Image.Dockerfile.DockerfileSrc.Uri != "" {
					dockerfileURI := component.Image.Dockerfile.DockerfileSrc.Uri
					absoluteURI := strings.HasPrefix(dockerfileURI, "http://") || strings.HasPrefix(dockerfileURI, "https://")
					if absoluteURI {
						// image uri
						_, err = CurlEndpoint(dockerfileURI)
					} else {
						if devfileSrc.Path != "" {
							// local devfile src with relative Dockerfile uri
							dockerfileURI = path.Join(path.Dir(URL), dockerfileURI)
							err = parserUtil.ValidateFile(dockerfileURI)
						} else {
							// remote devfile src with relative Dockerfile uri
							var u *url.URL
							u, err = url.Parse(URL)
							if err != nil {
								log.Error(err, fmt.Sprintf("failed to parse URL from %s", URL))
								return shouldIgnoreDevfile, nil, fmt.Errorf(fmt.Sprintf("failed to parse URL from %s", URL))
							}
							u.Path = path.Join(u.Path, dockerfileURI)
							dockerfileURI = u.String()
							_, err = CurlEndpoint(dockerfileURI)
						}
					}
					if err != nil {
						log.Error(err, fmt.Sprintf("failed to get Dockerfile from the URI %s, invalid image component: %s", URL, component.Name))
						return shouldIgnoreDevfile, nil, fmt.Errorf(fmt.Sprintf("failed to get Dockerfile from the URI %s, invalid image component: %s", URL, component.Name))
					}
				}
				if _, ok := deployCompMap[component.Name]; ok {
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("found more than one image components, but no deploy command associated with any being defined in the devfile from %s", URL)
				log.Error(err, "failed to validate devfile")
				return shouldIgnoreDevfile, nil, err
			}
		}
		// TODO: if only one image component, should return a warning that no apply command being defined
	}

	return shouldIgnoreDevfile, devfileBytes, nil
}

// DevfileSrc specifies the src of the Devfile
type DevfileSrc struct {
	Data string
	URL  string
	Path string
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
	} else if src.Path != "" {
		parserArgs.Path = src.Path
	} else {
		return nil, fmt.Errorf("cannot parse devfile without a src")
	}
	devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parserArgs)
	return devfileObj.Data, err
}

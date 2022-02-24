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

package devfile

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	devfilePkg "github.com/devfile/library/pkg/devfile"
	parser "github.com/devfile/library/pkg/devfile/parser"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	devfilev1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	mobyparser "github.com/moby/buildkit/frontend/dockerfile/parser"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/util"
)

const (
	DevfileName       = "devfile.yaml"
	HiddenDevfileName = ".devfile.yaml"
	HiddenDevfileDir  = ".devfile"

	Devfile                = DevfileName                                // devfile.yaml
	HiddenDevfile          = HiddenDevfileName                          // .devfile.yaml
	HiddenDirDevfile       = HiddenDevfileDir + "/" + DevfileName       // .devfile/devfile.yaml
	HiddenDirHiddenDevfile = HiddenDevfileDir + "/" + HiddenDevfileName // .devfile/.devfile.yaml
)

func ParseDevfileModel(devfileModel string) (data.DevfileData, error) {
	// Retrieve the devfile from the body of the resource
	devfileBytes := []byte(devfileModel)
	parserArgs := parser.ParserArgs{
		Data: devfileBytes,
	}
	devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parserArgs)
	return devfileObj.Data, err
}

// ConvertApplicationToDevfile takes in a given Application CR and converts it to
// a devfile object
func ConvertApplicationToDevfile(hasApp appstudiov1alpha1.Application, gitOpsRepo string, appModelRepo string) (data.DevfileData, error) {
	devfileVersion := string(data.APISchemaVersion210)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return nil, err
	}

	devfileData.SetSchemaVersion(devfileVersion)

	devfileAttributes := attributes.Attributes{}.PutString("gitOpsRepository.url", gitOpsRepo).PutString("appModelRepository.url", appModelRepo)

	// Add annotations for repo branch/contexts if needed
	if hasApp.Spec.AppModelRepository.Branch != "" {
		devfileAttributes.PutString("appModelRepository.branch", hasApp.Spec.AppModelRepository.Branch)
	}
	if hasApp.Spec.AppModelRepository.Context != "" {
		devfileAttributes.PutString("appModelRepository.context", hasApp.Spec.AppModelRepository.Context)
	}
	if hasApp.Spec.GitOpsRepository.Branch != "" {
		devfileAttributes.PutString("gitOpsRepository.branch", hasApp.Spec.GitOpsRepository.Branch)
	}
	if hasApp.Spec.GitOpsRepository.Context != "" {
		devfileAttributes.PutString("gitOpsRepository.context", hasApp.Spec.GitOpsRepository.Context)
	}

	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name:        hasApp.Spec.DisplayName,
		Description: hasApp.Spec.Description,
		Attributes:  devfileAttributes,
	})

	return devfileData, nil
}

// ConstructDevfileForComponent creates devfile for given component
func ConstructDevfileForComponent(component *appstudiov1alpha1.Component, appFs afero.Fs, rootDir string, pipelineName string) (string, error) {
	switch pipelineName {
	case DevfileBuild:
		// It is safe to ignore errors below because DetermineBuildPipeline checked that a devfile exists
		devfiles, _ := ReadDevfilesFromRepo(rootDir, 1)
		for _, devfileBytes := range devfiles {
			return string(devfileBytes), nil
		}
	case DockerBuild:
		dockerfileBytes, err := ioutil.ReadFile(path.Join(rootDir, DockerfileName))
		if err != nil {
			return "", err
		}
		return constructDevfileFromDockerfile(component, string(dockerfileBytes))
	case NoOpBuild:
		return constructEmptyDevfile(component)
	}
	return "", fmt.Errorf("unknown pipeline %s", pipelineName)
}

func constructDevfileFromDockerfile(component *appstudiov1alpha1.Component, dockerfile string) (string, error) {
	// Parse dockerfile using original parser
	parsedDockerfile, err := mobyparser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		return "", fmt.Errorf("failed to parse dockerfile: %s", err.Error())
	}
	dockerfileRootNode := parsedDockerfile.AST

	// Construct devfile
	devfileVersion := string(data.APISchemaVersion220)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return "", err
	}

	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name: component.Name,
	})

	// Add project
	if component.Spec.Source.GitSource != nil && component.Spec.Source.GitSource.URL != "" {
		project := devfilev1.Project{
			ProjectSource: devfilev1.ProjectSource{
				Git: &devfilev1.GitProjectSource{
					GitLikeProjectSource: devfilev1.GitLikeProjectSource{
						Remotes: map[string]string{
							"origin": component.Spec.Source.GitSource.URL,
						},
					},
				},
			},
		}

		if err := devfileData.AddProjects([]devfilev1.Project{project}); err != nil {
			return "", err
		}
	}

	// Get exposed endpoints
	var endpoints []devfilev1.Endpoint
	for _, directiveRootNode := range dockerfileRootNode.Children {
		if strings.ToUpper(directiveRootNode.Value) == "EXPOSE" {
			for node := directiveRootNode.Next; node != nil; node = node.Next {
				// value contains exposed port or port/protocol, e.g. 1234 or 5000/tcp
				value := node.Value

				var port int
				protocol := ""
				if strings.Contains(value, "/") {
					portProtocol := strings.Split(value, "/")
					port, err = strconv.Atoi(portProtocol[0])
					if err != nil {
						continue
					}
					protocol = portProtocol[1]
				} else {
					port, err = strconv.Atoi(value)
					if err != nil {
						continue
					}
				}

				endpoints = append(endpoints, devfilev1.Endpoint{
					TargetPort: port,
					Protocol:   devfilev1.EndpointProtocol(protocol),
					Exposure:   "public",
				})
			}
		}
	}

	// Add component based on build from the dockerfile image
	devfileComponent := devfilev1.Component{
		Name: component.Name,
		ComponentUnion: devfilev1.ComponentUnion{
			Container: &devfilev1.ContainerComponent{
				Container: devfilev1.Container{
					Image: component.Status.ContainerImage,
				},
				Endpoints: endpoints,
			},
		},
	}
	devfileData.AddComponents([]devfilev1.Component{devfileComponent})

	devfileYaml, err := yaml.Marshal(devfileData)
	if err != nil {
		return "", err
	}
	return string(devfileYaml), nil
}

func constructEmptyDevfile(component *appstudiov1alpha1.Component) (string, error) {
	devfileVersion := string(data.APISchemaVersion220)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return "", err
	}
	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name: component.Name,
	})
	devfileYaml, err := yaml.Marshal(devfileData)
	if err != nil {
		return "", err
	}
	return string(devfileYaml), nil
}

// DownloadDevfile downloads devfile from the various possible devfile locations in dir and returns the contents
func DownloadDevfile(dir string) ([]byte, error) {
	var devfileBytes []byte
	var err error
	validDevfileLocations := []string{Devfile, HiddenDevfile, HiddenDirDevfile, HiddenDirHiddenDevfile}

	for _, path := range validDevfileLocations {
		devfilePath := dir + "/" + path
		devfileBytes, err = util.CurlEndpoint(devfilePath)
		if err == nil {
			// if we get a 200, return
			return devfileBytes, err
		}
	}

	return nil, fmt.Errorf("unable to find any devfiles in dir %s", dir)
}

// ReadDevfilesFromRepo attempts to read and return devfiles from the local path upto the specified depth
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
		if (f.Name() == DevfileName || f.Name() == HiddenDevfileName) && currentLevel != 0 {
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
		} else if f.IsDir() && f.Name() == HiddenDevfileDir {
			// if the dir is .devfile, we dont increment currentLevel
			// consider devfile.yaml and .devfile/devfile.yaml as the same level
			recursiveMap, err := searchDevfiles(path.Join(localpath, f.Name()), currentLevel, depth)
			if err != nil {
				return nil, err
			}

			var context string
			currentPath := localpath
			for i := 0; i < currentLevel; i++ {
				context = path.Join(filepath.Base(currentPath), context)
				currentPath = filepath.Dir(currentPath)
			}
			for recursiveContext := range recursiveMap {
				if recursiveContext == HiddenDevfileDir {
					devfileMapFromRepo[context] = recursiveMap[HiddenDevfileDir]
				}
			}
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

	// if we didnt find any devfile we should return an err
	if len(devfileMapFromRepo) == 0 && currentLevel == 0 {
		err = fmt.Errorf("unable to find any devfile(s) in the multi component repo, devfiles can be detected only upto a depth of %v dir", depth)
	}

	return devfileMapFromRepo, err
}

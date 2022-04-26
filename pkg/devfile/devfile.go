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
	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	devfilePkg "github.com/devfile/library/pkg/devfile"
	parser "github.com/devfile/library/pkg/devfile/parser"
	data "github.com/devfile/library/pkg/devfile/parser/data"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/util"

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
	} else {
		devfileAttributes.PutString("appModelRepository.context", "/")
	}
	if hasApp.Spec.GitOpsRepository.Branch != "" {
		devfileAttributes.PutString("gitOpsRepository.branch", hasApp.Spec.GitOpsRepository.Branch)
	}
	if hasApp.Spec.GitOpsRepository.Context != "" {
		devfileAttributes.PutString("gitOpsRepository.context", hasApp.Spec.GitOpsRepository.Context)
	} else {
		devfileAttributes.PutString("gitOpsRepository.context", "/")
	}

	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name:        hasApp.Spec.DisplayName,
		Description: hasApp.Spec.Description,
		Attributes:  devfileAttributes,
	})

	return devfileData, nil
}

func ConvertImageComponentToDevfile(comp appstudiov1alpha1.Component) (data.DevfileData, error) {
	devfileVersion := string(data.APISchemaVersion210)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return nil, err
	}

	devfileData.SetSchemaVersion(devfileVersion)
	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name: comp.Spec.ComponentName,
	})

	// Generate a stub container component for the devfile
	components := []v1alpha2.Component{
		{
			Name: "container",
			ComponentUnion: v1alpha2.ComponentUnion{
				Container: &v1alpha2.ContainerComponent{
					Container: v1alpha2.Container{
						Image: comp.Spec.Source.ImageSource.ContainerImage,
					},
				},
			},
		},
	}

	devfileData.AddComponents(components)

	return devfileData, nil
}

func CreateDevfileForDockerfileBuild(uri, context string) (data.DevfileData, error) {
	devfileVersion := string(data.APISchemaVersion220)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return nil, err
	}

	devfileData.SetSchemaVersion(devfileVersion)

	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name:        "dockerfile-component",
		Description: "Basic Devfile for a Dockerfile Component",
	})

	components := []v1alpha2.Component{
		{
			Name: "dockerfile-build",
			ComponentUnion: v1alpha2.ComponentUnion{
				Image: &v1alpha2.ImageComponent{
					Image: v1alpha2.Image{
						ImageUnion: v1alpha2.ImageUnion{
							Dockerfile: &v1alpha2.DockerfileImage{
								DockerfileSrc: v1alpha2.DockerfileSrc{
									Uri: uri,
								},
								Dockerfile: v1alpha2.Dockerfile{
									BuildContext: context,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "container",
			ComponentUnion: v1alpha2.ComponentUnion{
				Container: &v1alpha2.ContainerComponent{
					Container: v1alpha2.Container{
						Image: "no-op",
					},
				},
			},
		},
	}
	err = devfileData.AddComponents(components)
	if err != nil {
		return nil, err
	}

	commands := []v1alpha2.Command{
		{
			Id: "build-image",
			CommandUnion: v1alpha2.CommandUnion{
				Apply: &v1alpha2.ApplyCommand{
					Component: "dockerfile-build",
				},
			},
		},
	}
	err = devfileData.AddCommands(commands)
	if err != nil {
		return nil, err
	}

	return devfileData, nil
}

// DownloadDevfile downloads devfile from the various possible devfile locations in dir and returns the contents
func DownloadDevfile(dir string) ([]byte, error) {
	var devfileBytes []byte
	var err error
	validDevfileLocations := []string{Devfile, HiddenDevfile, HiddenDirDevfile, HiddenDirHiddenDevfile}

	for _, path := range validDevfileLocations {
		devfilePath := dir + "/" + path
		devfileBytes, err = DownloadFile(devfilePath)
		if err == nil {
			// if we get a 200, return
			return devfileBytes, err
		}
	}

	return nil, &NoDevfileFound{Location: dir}
}

// DownloadFile downloads the specified file
func DownloadFile(file string) ([]byte, error) {
	return util.CurlEndpoint(file)
}

// DownloadDevfileAndDockerfile attempts to download the  devfile and dockerfile from the root of the specified url
func DownloadDevfileAndDockerfile(url string) ([]byte, []byte) {
	var devfileBytes, dockerfileBytes []byte

	devfileBytes, _ = DownloadDevfile(url)
	dockerfileBytes, _ = DownloadFile(url + "/Dockerfile")

	return devfileBytes, dockerfileBytes
}

// ScanRepo attempts to read and return devfiles and dockerfiles from the local path upto the specified depth
// If no devfile(s) or dockerfile(s) are found, then the Alizer tool is used to detect and match a devfile/dockerfile from the devfile registry
// ScanRepo returns 3 maps and an error:
// Map 1 returns a context to the devfile bytes if present.
// Map 2 returns a context to the matched devfileURL from the devfile registry if no devfile is present in the context.
// Map 3 returns a context to the dockerfile uri or a matched dockerfileURL from the devfile registry if no dockerfile is present in the context
func ScanRepo(log logr.Logger, a Alizer, localpath string, depth int, devfileRegistryURL string) (map[string][]byte, map[string]string, map[string]string, error) {
	return search(log, a, localpath, 0, depth, devfileRegistryURL)
}

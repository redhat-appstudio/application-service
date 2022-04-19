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
)

const (
	DevfileName       = "devfile.yaml"
	HiddenDevfileName = ".devfile.yaml"
	HiddenDevfileDir  = ".devfile"

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

	return nil, &NoDevfileFound{location: dir}
}

// ReadDevfilesFromRepo attempts to read and return devfiles from the local path upto the specified depth
// If no devfile(s) is found, then the Alizer tool is used to detect and match a devfile from the devfile registry
func ReadDevfilesFromRepo(a Alizer, localpath string, depth int, devfileRegistryURL string) (map[string][]byte, map[string]string, error) {
	return searchDevfiles(a, localpath, 0, depth, devfileRegistryURL)
}

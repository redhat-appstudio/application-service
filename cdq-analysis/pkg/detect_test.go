//
// Copyright 2022-2023 Red Hat, Inc.
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
	"os"
	"reflect"
	"testing"

	"github.com/redhat-developer/alizer/go/pkg/apis/model"
)

func TestAnalyzeAndDetectDevfile(t *testing.T) {

	var mockClient MockAlizerClient

	tests := []struct {
		name                string
		clonePath           string
		repo                string
		revision            string
		token               string
		registryURL         string
		wantDevfile         bool
		wantDevfileEndpoint string
		wantDetectedPorts   []int
		wantErr             bool
	}{
		{
			name:                "Successfully detect a devfile from the registry",
			clonePath:           "/tmp/java-springboot-basic",
			repo:                "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			registryURL:         DevfileStageRegistryEndpoint,
			wantDevfile:         true,
			wantDevfileEndpoint: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
		},
		{
			name:                "Successfully detect a devfile from the registry using an alternate branch",
			clonePath:           "/tmp/java-springboot-basic",
			repo:                "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			revision:            "testbranch",
			registryURL:         DevfileStageRegistryEndpoint,
			wantDevfile:         true,
			wantDevfileEndpoint: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
		},
		{
			name:        "Cannot detect a devfile for a Scala repository",
			clonePath:   "/tmp/testscala",
			repo:        "https://github.com/johnmcollier/scalatemplate",
			registryURL: DevfileStageRegistryEndpoint,
			wantErr:     true,
		},
		{
			name:        "Test err condition for Alizer Analyze",
			clonePath:   "/tmp/errorAnalyze",
			repo:        "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			registryURL: DevfileStageRegistryEndpoint,
			wantErr:     true,
		},
		{
			name:        "Test with a fake Devfile Registry",
			clonePath:   "/tmp/path",
			repo:        "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			registryURL: "FakeDevfileRegistryEndpoint",
			wantErr:     true,
		},
		{
			name:        "Test err condition for Alizer SelectDevFileFromTypes",
			clonePath:   "/tmp/springboot/errorSelectDevFileFromTypes",
			repo:        "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			registryURL: DevfileStageRegistryEndpoint,
			wantErr:     true,
		},
		{
			name:        "Test err condition for failing to hit the devfile endpoint",
			clonePath:   "/tmp/springboot/error/devfileendpoint",
			repo:        "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			registryURL: DevfileStageRegistryEndpoint,
			wantErr:     true,
		},
		{
			name:                "Component detected successfully with Port(s) detected",
			clonePath:           "/tmp/nodejsports-devfile-sample-nodejs-basic",
			repo:                "https://github.com/devfile-resources/node-express-hello-no-devfile",
			registryURL:         DevfileStageRegistryEndpoint,
			wantDevfile:         true,
			wantDevfileEndpoint: "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml",
			wantDetectedPorts:   []int{8080},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneRepo(tt.clonePath, GitURL{
				RepoURL:  tt.repo,
				Revision: tt.revision,
				Token:    tt.token,
			})
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				devfileBytes, detectedDevfileEndpoint, _, detectedPorts, err := AnalyzeAndDetectDevfile(mockClient, tt.clonePath, tt.registryURL)
				if !tt.wantErr && err != nil {
					t.Errorf("Unexpected err: %+v", err)
				} else if tt.wantErr && err == nil {
					t.Errorf("Expected error but got nil")
				} else if !reflect.DeepEqual(len(devfileBytes) > 0, tt.wantDevfile) {
					t.Errorf("Expected devfile: %+v, Got: %+v, devfile %+v", tt.wantDevfile, len(devfileBytes) > 0, string(devfileBytes))
				} else if !reflect.DeepEqual(detectedDevfileEndpoint, tt.wantDevfileEndpoint) {
					t.Errorf("Expected devfile endpoint: %+v, Got: %+v", tt.wantDevfileEndpoint, detectedDevfileEndpoint)
				} else if !reflect.DeepEqual(detectedPorts, tt.wantDetectedPorts) {
					t.Errorf("Expected detected ports: %+v, Got: %+v", tt.wantDetectedPorts, detectedPorts)
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

func TestSelectDevfileFromTypes(t *testing.T) {

	var alizerClient AlizerClient

	tests := []struct {
		name            string
		clonePath       string
		repo            string
		devfileTypes    []model.DevFileType
		wantErr         bool
		wantDevfileType model.DevFileType
	}{
		{
			name:      "Successfully detect a devfile from the registry",
			clonePath: "/tmp/test-selected-devfile",
			repo:      "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			devfileTypes: []model.DevFileType{
				{
					Name: "nodejs-basic", Language: "JavaScript", ProjectType: "Node.js", Tags: []string{"Node.js", "Express"},
				},
				{
					Name: "code-with-quarkus", Language: "Java", ProjectType: "Quarkus", Tags: []string{"Java", "Quarkus"},
				},
				{
					Name: "java-springboot-basic", Language: "Java", ProjectType: "springboot", Tags: []string{"Java", "Spring"},
				},
				{
					Name: "python-basic", Language: "Python", ProjectType: "Python", Tags: []string{"Python", "Pip", "Flask"},
				},
				{
					Name: "go-basic", Language: "Go", ProjectType: "Go", Tags: []string{"Go"},
				},
				{
					Name: "dotnet-basic", Language: ".NET", ProjectType: "dotnet", Tags: []string{".NET"},
				},
			},
			wantErr: false,
			wantDevfileType: model.DevFileType{
				Name: "java-springboot-basic", Language: "Java", ProjectType: "springboot", Tags: []string{"Java", "Spring"},
			},
		},
		{
			name:      "Unable to detect a devfile from the registry",
			clonePath: "/tmp/test-no-devfiles-selected",
			repo:      "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			devfileTypes: []model.DevFileType{
				{
					Name: "python-basic", Language: "Python", ProjectType: "Python", Tags: []string{"Python", "Pip", "Flask"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.RemoveAll(tt.clonePath)
			err := CloneRepo(tt.clonePath, GitURL{
				RepoURL:  tt.repo,
				Revision: "main",
				Token:    "",
			})
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			}

			devfileType, err := alizerClient.SelectDevFileFromTypes(tt.clonePath, tt.devfileTypes)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			}

			if !tt.wantErr {
				if !reflect.DeepEqual(devfileType, tt.wantDevfileType) {
					t.Errorf("Expected devfileType: %v, got %v", tt.wantDevfileType, devfileType)
				}
			}
		})
	}
}

func TestSearchForDockerfile(t *testing.T) {

	tests := []struct {
		name          string
		devfileString string
		found         bool
		wantErr       bool
	}{
		{
			name: "Successfully get the Devfile Uri",
			devfileString: `
schemaVersion: 2.2.0
metadata:
  name: nodejs
components:
  - name: outerloop-build
    image:
      imageName: nodejs-image:latest
      dockerfile:
        uri: "myuri"`,
			found: true,
		},
		{
			name: "No Devfile Uri",
			devfileString: `
schemaVersion: 2.2.0
metadata:
  name: nodejs
components:
  - name: outerloop-build
    image:
      imageName: nodejs-image:latest
      dockerfile:
        uri: ""`,
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileBytes := []byte(tt.devfileString)
			dockerfileImage, err := SearchForDockerfile(devfileBytes, "")
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if tt.found && dockerfileImage == nil {
				t.Errorf("dockerfile should be found, but got %v", dockerfileImage)
			} else if !tt.found && dockerfileImage != nil {
				t.Errorf("dockerfile should be found, but got %v", dockerfileImage)
			}
		})
	}
}

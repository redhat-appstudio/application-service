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
	"os"
	"reflect"
	"testing"

	"github.com/redhat-appstudio/application-service/pkg/util"
)

func TestAnalyzeAndDetectDevfile(t *testing.T) {

	var mockClient MockAlizerClient

	tests := []struct {
		name                string
		clonePath           string
		repo                string
		token               string
		registryURL         string
		wantDevfile         bool
		wantDevfileEndpoint string
		wantErr             bool
	}{
		{
			name:                "Successfully detect a devfile from the registry",
			clonePath:           "/tmp/java-springboot-basic",
			repo:                "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			registryURL:         DevfileStageRegistryEndpoint,
			wantDevfile:         true,
			wantDevfileEndpoint: "https://registry.stage.devfile.io/devfiles/java-springboot-basic",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := util.CloneRepo(tt.clonePath, tt.repo, tt.token)
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				devfileBytes, detectedDevfileEndpoint, _, err := AnalyzeAndDetectDevfile(mockClient, tt.clonePath, tt.registryURL)
				if !tt.wantErr && err != nil {
					t.Errorf("Unexpected err: %+v", err)
				} else if tt.wantErr && err == nil {
					t.Errorf("Expected error but got nil")
				} else if !reflect.DeepEqual(len(devfileBytes) > 0, tt.wantDevfile) {
					t.Errorf("Expected devfile: %+v, Got: %+v, devfile %+v", tt.wantDevfile, len(devfileBytes) > 0, string(devfileBytes))
				} else if !reflect.DeepEqual(detectedDevfileEndpoint, tt.wantDevfileEndpoint) {
					t.Errorf("Expected devfile endpoint: %+v, Got: %+v", tt.wantDevfileEndpoint, detectedDevfileEndpoint)
				}
			}
			os.RemoveAll(tt.clonePath)
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
			dockerfileImage, err := SearchForDockerfile(devfileBytes)
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

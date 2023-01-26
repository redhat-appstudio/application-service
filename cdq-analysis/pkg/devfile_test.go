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
	"os"
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestParseDevfileModel(t *testing.T) {
	tests := []struct {
		name          string
		devfileString string
		wantDevfile   *v2.DevfileV2
	}{
		{
			name: "Simple HASApp CR",
			devfileString: `
metadata:
  attributes:
    appModelRepository.url: https://github.com/testorg/petclinic-app
    gitOpsRepository.url: https://github.com/testorg/petclinic-gitops
  name: petclinic
schemaVersion: 2.2.0`,
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			devfile, err := ParseDevfileModel(tt.devfileString)
			if err != nil {
				t.Errorf("TestConvertApplicationToDevfile() unexpected error: %v", err)
			} else if !reflect.DeepEqual(devfile, tt.wantDevfile) {
				t.Errorf("TestConvertApplicationToDevfile() error: expected %v got %v", tt.wantDevfile, devfile)
			}
		})
	}
}

func TestFindAndDownloadDevfile(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		wantDevfileContext string
		wantErr            bool
	}{
		{
			name:               "Curl devfile.yaml",
			url:                "https://raw.githubusercontent.com/maysunfaisal/devfilepriority/main/case1",
			wantDevfileContext: "devfile.yaml",
		},
		{
			name:               "Curl .devfile.yaml",
			url:                "https://raw.githubusercontent.com/maysunfaisal/devfilepriority/main/case2",
			wantDevfileContext: ".devfile.yaml",
		},
		{
			name:               "Curl .devfile/devfile.yaml",
			url:                "https://raw.githubusercontent.com/maysunfaisal/devfilepriority/main/case3",
			wantDevfileContext: ".devfile/devfile.yaml",
		},
		{
			name:               "Curl .devfile/.devfile.yaml",
			url:                "https://raw.githubusercontent.com/maysunfaisal/devfilepriority/main/case4",
			wantDevfileContext: ".devfile/.devfile.yaml",
		},
		{
			name:    "Cannot curl for a devfile",
			url:     "https://github.com/octocat/Hello-World",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, devfileContext, err := FindAndDownloadDevfile(tt.url)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil && contents == nil {
				t.Errorf("unable to read body")
			} else if err == nil && (devfileContext != tt.wantDevfileContext) {
				t.Errorf("devfile context did not match, got %v, wanted %v", devfileContext, tt.wantDevfileContext)
			}
		})
	}
}

func TestDownloadDevfileAndDockerfile(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		wantDevfileContext string
		want               bool
	}{
		{
			name:               "Curl devfile.yaml and dockerfile",
			url:                "https://raw.githubusercontent.com/maysunfaisal/devfile-sample-python-samelevel/main",
			wantDevfileContext: ".devfile.yaml",
			want:               true,
		},
		{
			name: "Cannot curl for a devfile nor a dockerfile",
			url:  "https://github.com/octocat/Hello-World",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfile, devfileContext, dockerfile := DownloadDevfileAndDockerfile(tt.url)
			if tt.want != (len(devfile) > 0 && len(dockerfile) > 0) {
				t.Errorf("devfile and a dockerfile wanted: %v but got devfile: %v dockerfile: %v", tt.want, len(devfile) > 0, len(dockerfile) > 0)
			}

			if devfileContext != tt.wantDevfileContext {
				t.Errorf("devfile context did not match, got %v, wanted %v", devfileContext, tt.wantDevfileContext)
			}
		})
	}
}

func TestScanRepo(t *testing.T) {

	var logger logr.Logger
	var alizerClient AlizerClient // Use actual client because this is a huge wrapper function and mocking so many possibilities is pretty tedious when everything is changing frequently

	tests := []struct {
		name                         string
		clonePath                    string
		repo                         string
		token                        string
		wantErr                      bool
		expectedDevfileContext       []string
		expectedDevfileURLContextMap map[string]string
		expectedDockerfileContextMap map[string]string
	}{
		{
			name:                   "Should return 2 devfile contexts, and 2 devfileURLs as this is a multi comp devfile",
			clonePath:              "/tmp/testclone",
			repo:                   "https://github.com/maysunfaisal/multi-components-deep",
			expectedDevfileContext: []string{"python", "devfile-sample-java-springboot-basic"},
			expectedDevfileURLContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-deep/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
				"python":                               "https://registry.stage.devfile.io/devfiles/python-basic",
			},
			expectedDockerfileContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "devfile-sample-java-springboot-basic/docker/Dockerfile",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"},
		},
		{
			name:                   "Should return 4 devfiles, 5 devfile url and 5 dockerfile uri as this is a multi comp devfile",
			clonePath:              "/tmp/testclone",
			repo:                   "https://github.com/maysunfaisal/multi-components-dockerfile",
			expectedDevfileContext: []string{"devfile-sample-java-springboot-basic", "devfile-sample-nodejs-basic", "devfile-sample-python-basic", "python-src-none"},
			expectedDevfileURLContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
				"devfile-sample-nodejs-basic":          "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-nodejs-basic/devfile.yaml",
				"devfile-sample-python-basic":          "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-python-basic/.devfile.yaml",
				"python-src-none":                      "https://registry.stage.devfile.io/devfiles/python-basic",
				"python-src-docker":                    "https://registry.stage.devfile.io/devfiles/python-basic",
			},
			expectedDockerfileContextMap: map[string]string{
				"python-src-docker":                    "python-src-docker/Dockerfile",
				"devfile-sample-nodejs-basic":          "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile",
				"devfile-sample-java-springboot-basic": "devfile-sample-java-springboot-basic/docker/Dockerfile",
				"python-src-none":                      "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile",
				"devfile-sample-python-basic":          "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-python-basic/Dockerfile"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger = ctrl.Log.WithName("TestScanRepo")
			err := CloneRepo(tt.clonePath, tt.repo, tt.token)
			URL:=tt.repo
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				devfileMap, devfileURLMap, dockerfileMap, err := ScanRepo(logger, alizerClient, tt.clonePath, DevfileStageRegistryEndpoint, URL, "", "")
				if tt.wantErr && (err == nil) {
					t.Error("wanted error but got nil")
				} else if !tt.wantErr && err != nil {
					t.Errorf("got unexpected error %v", err)
				} else {
					for actualContext := range devfileMap {
						matched := false
						for _, expectedContext := range tt.expectedDevfileContext {
							if expectedContext == actualContext {
								matched = true
								break
							}
						}

						if !matched {
							t.Errorf("found devfile at context %v but expected none", actualContext)
						}
					}

					for actualContext := range devfileURLMap {
						if devfileURLMap[actualContext] != tt.expectedDevfileURLContextMap[actualContext] {
							t.Errorf("expected devfile URL %v but got %v", tt.expectedDevfileURLContextMap[actualContext], devfileURLMap[actualContext])
						}

					}

					for actualContext := range dockerfileMap {
						if tt.expectedDockerfileContextMap[actualContext] != dockerfileMap[actualContext] {
							t.Errorf("found dockerfile uri at context %v:%v but expected %v", actualContext, dockerfileMap[actualContext], tt.expectedDockerfileContextMap[actualContext])
						}
					}
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

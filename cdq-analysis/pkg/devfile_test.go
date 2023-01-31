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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestParseDevfile(t *testing.T) {

	testServerURL := "127.0.0.1:9080"

	simpleDevfile := `
metadata:
  attributes:
    appModelRepository.url: https://github.com/testorg/petclinic-app
    gitOpsRepository.url: https://github.com/testorg/petclinic-gitops
  name: petclinic
schemaVersion: 2.2.0`

	testServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(simpleDevfile))
		if err != nil {
			t.Errorf("TestParseDevfileModel() unexpected error while writing data: %v", err)
		}
	}))
	// create a listener with the desired port.
	l, err := net.Listen("tcp", testServerURL)
	if err != nil {
		t.Errorf("TestParseDevfileModel() unexpected error while creating listener: %v", err)
		return
	}

	// NewUnstartedServer creates a listener. Close that listener and replace
	// with the one we created.
	testServer.Listener.Close()
	testServer.Listener = l

	testServer.Start()
	defer testServer.Close()

	tests := []struct {
		name              string
		devfileString     string
		devfileURL        string
		wantDevfile       *v2.DevfileV2
		wantMetadata      devfile.DevfileMetadata
		wantSchemaVersion string
	}{
		{
			name:          "Simple devfile from data",
			devfileString: simpleDevfile,
			wantMetadata: devfile.DevfileMetadata{
				Name:       "petclinic",
				Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
			},
			wantSchemaVersion: string(data.APISchemaVersion220),
		},
		{
			name:       "Simple devfile from URL",
			devfileURL: "http://" + testServerURL,
			wantMetadata: devfile.DevfileMetadata{
				Name:       "petclinic",
				Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
			},
			wantSchemaVersion: string(data.APISchemaVersion220),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var devfileSrc DevfileSrc
			if tt.devfileString != "" {
				devfileSrc = DevfileSrc{
					Data: tt.devfileString,
				}
			} else if tt.devfileURL != "" {
				devfileSrc = DevfileSrc{
					URL: tt.devfileURL,
				}
			}
			devfile, err := ParseDevfile(devfileSrc)
			if err != nil {
				t.Errorf("TestParseDevfileModel() unexpected error: %v", err)
			} else {
				gotMetadata := devfile.GetMetadata()
				if !reflect.DeepEqual(gotMetadata, tt.wantMetadata) {
					t.Errorf("TestParseDevfileModel() metadata is different")
				}

				gotSchemaVersion := devfile.GetSchemaVersion()
				if gotSchemaVersion != tt.wantSchemaVersion {
					t.Errorf("TestParseDevfileModel() schema version is different")
				}
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
			URL := tt.repo
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

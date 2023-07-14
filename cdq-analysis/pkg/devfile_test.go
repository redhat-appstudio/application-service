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
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	devfilePkg "github.com/devfile/library/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestParseDevfileModel(t *testing.T) {

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

	localPath := "/tmp/testDir"
	localDevfilePath := path.Join(localPath, "devfile.yaml")
	// prepare for local file
	err = os.MkdirAll(localPath, 0755)
	if err != nil {
		t.Errorf("TestParseDevfileModel() error: failed to create folder: %v, error: %v", localPath, err)
	}
	err = ioutil.WriteFile(localDevfilePath, []byte(simpleDevfile), 0644)
	if err != nil {
		t.Errorf("TestParseDevfileModel() error: fail to write to file: %v", err)
	}

	if err != nil {
		t.Error(err)
	}

	defer os.RemoveAll(localPath)

	tests := []struct {
		name              string
		devfileString     string
		devfileURL        string
		devfilePath       string
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
		{
			name:        "Simple devfile from PATH",
			devfilePath: localDevfilePath,
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
			} else if tt.devfilePath != "" {
				devfileSrc = DevfileSrc{
					Path: tt.devfilePath,
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
		revision                     string
		token                        string
		wantErr                      bool
		expectedDevfileContext       []string
		expectedDevfileURLContextMap map[string]string
		expectedDockerfileContextMap map[string]string
		expectedPortsMap             map[string][]int
	}{
		{
			name:                   "Should return 2 devfile contexts, and 2 devfileURLs as this is a multi comp devfile",
			clonePath:              "/tmp/testclone",
			repo:                   "https://github.com/maysunfaisal/multi-components-deep",
			expectedDevfileContext: []string{"python", "devfile-sample-java-springboot-basic"},
			expectedDevfileURLContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-deep/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
			},
			expectedDockerfileContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "devfile-sample-java-springboot-basic/docker/Dockerfile",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"},
			expectedPortsMap: map[string][]int{
				"python": {8081},
			},
		},
		{
			name:                   "Should return 2 devfile contexts, and 2 devfileURLs as this is a multi comp devfile - with revision specified",
			clonePath:              "/tmp/testclone",
			repo:                   "https://github.com/maysunfaisal/multi-components-deep",
			revision:               "main",
			expectedDevfileContext: []string{"python", "devfile-sample-java-springboot-basic"},
			expectedDevfileURLContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-deep/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
			},
			expectedDockerfileContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "devfile-sample-java-springboot-basic/docker/Dockerfile",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"},
			expectedPortsMap: map[string][]int{
				"python": {8081},
			},
		},
		{
			name:                   "Should return 2 devfile contexts, and 2 devfileURLs with multi-component but no outerloop definition",
			clonePath:              "/tmp/testclone",
			repo:                   "https://github.com/yangcao77/multi-components-with-no-kubecomps",
			expectedDevfileContext: []string{"python", "devfile-sample-java-springboot-basic"},
			expectedDevfileURLContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
			},
			expectedDockerfileContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
				"python":                               "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"},
		},
		{
			name:                   "Should return 4 devfiles, 5 devfile url and 5 Dockerfile uri as this is a multi comp devfile",
			clonePath:              "/tmp/testclone",
			repo:                   "https://github.com/maysunfaisal/multi-components-dockerfile",
			expectedDevfileContext: []string{"devfile-sample-java-springboot-basic", "devfile-sample-nodejs-basic", "devfile-sample-python-basic", "python-src-none"},
			expectedDevfileURLContextMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
				"devfile-sample-nodejs-basic":          "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml",
				"devfile-sample-python-basic":          "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-python-basic/.devfile.yaml",
				"python-src-none":                      "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
				"python-src-docker":                    "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
			},
			expectedDockerfileContextMap: map[string]string{
				"python-src-docker":                    "Dockerfile",
				"devfile-sample-nodejs-basic":          "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile",
				"devfile-sample-java-springboot-basic": "devfile-sample-java-springboot-basic/docker/Dockerfile",
				"python-src-none":                      "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile",
				"devfile-sample-python-basic":          "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-python-basic/Dockerfile"},
			expectedPortsMap: map[string][]int{
				"devfile-sample-nodejs-basic": {3000},
			},
		},
		{
			name:      "Should return 4 Dockerfile contexts with Dockerfile/Containerfile path, and 4 devfileURLs ",
			clonePath: "/tmp/testclone",
			repo:      "https://github.com/yangcao77/multi-components-dockerfile",
			revision:  "containerfile",
			expectedDevfileURLContextMap: map[string]string{
				"java-springboot-containerfile": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
				"java-springboot-dockerfile":    "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
				"python-dockerfile":             "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
				"python-containerfile":          "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
			},
			expectedDockerfileContextMap: map[string]string{
				"java-springboot-dockerfile":    "docker/Dockerfile",
				"java-springboot-containerfile": "docker/Containerfile",
				"python-dockerfile":             "docker/Dockerfile",
				"python-containerfile":          "Containerfile"},
		},
		{
			name:                   "Should return one context with one devfile, along with one port detected",
			clonePath:              "/tmp/testclonenode-devfile-sample-nodejs-basic",
			repo:                   "https://github.com/devfile-resources/single-component-port-detected",
			expectedDevfileContext: []string{"nodejs"},
			expectedDevfileURLContextMap: map[string]string{
				"nodejs": "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml",
			},
			expectedDockerfileContextMap: map[string]string{
				"nodejs": "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile",
			},
			expectedPortsMap: map[string][]int{
				"nodejs": {8080},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger = ctrl.Log.WithName("TestScanRepo")
			err := CloneRepo(tt.clonePath, tt.repo, tt.revision, tt.token)
			URL := tt.repo
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				devfileMap, devfileURLMap, dockerfileMap, portsMap, err := ScanRepo(logger, alizerClient, tt.clonePath, DevfileStageRegistryEndpoint, URL, "", "")
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
							t.Errorf("found Dockerfile uri at context %v:%v but expected %v", actualContext, dockerfileMap[actualContext], tt.expectedDockerfileContextMap[actualContext])
						}
					}

					for actualContext := range portsMap {
						if !reflect.DeepEqual(tt.expectedPortsMap[actualContext], portsMap[actualContext]) {
							t.Errorf("found port(s) at context %v:%v but expected %v", actualContext, portsMap[actualContext], tt.expectedPortsMap[actualContext])
						}
					}
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

func TestValidateDevfile(t *testing.T) {
	logger := ctrl.Log.WithName("TestValidateDevfile")
	httpTimeout := 10
	convert := true
	parserArgs := parser.ParserArgs{
		HTTPTimeout:                   &httpTimeout,
		ConvertKubernetesContentInUri: &convert,
	}

	springDevfileParser := parserArgs
	springDevfileParser.URL = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"

	springDevfileObj, _, err := devfilePkg.ParseDevfileAndValidate(springDevfileParser)
	if err != nil {
		t.Errorf("TestValidateDevfile() unexpected error: %v", err)
	}
	springDevfileBytes, err := yaml.Marshal(springDevfileObj.Data)
	if err != nil {
		t.Errorf("TestValidateDevfile() unexpected error: %v", err)
	}

	springDevfileWithAbsoluteDockerfileParser := parserArgs
	springDevfileWithAbsoluteDockerfileParser.URL = "https://raw.githubusercontent.com/yangcao77/spring-sample-with-absolute-dockerfileURI/main/devfile.yaml"
	springDevfileObjWithAbsoluteDockerfile, _, err := devfilePkg.ParseDevfileAndValidate(springDevfileWithAbsoluteDockerfileParser)
	if err != nil {
		t.Errorf("TestValidateDevfile() unexpected error: %v", err)
	}
	springDevfileWithAbsoluteDockerfileBytes, err := yaml.Marshal(springDevfileObjWithAbsoluteDockerfile.Data)
	if err != nil {
		t.Errorf("TestValidateDevfile() unexpected error: %v", err)
	}

	tests := []struct {
		name             string
		url              string
		wantDevfileBytes []byte
		wantIgnore       bool
		wantErr          bool
	}{
		{
			name:             "should success with valid deploy.yaml URI and relative Dockerfile URI references",
			url:              springDevfileParser.URL,
			wantDevfileBytes: springDevfileBytes,
			wantIgnore:       false,
			wantErr:          false,
		},
		{
			name:             "should success with valid Dockerfile absolute URL references",
			url:              springDevfileWithAbsoluteDockerfileParser.URL,
			wantDevfileBytes: springDevfileWithAbsoluteDockerfileBytes,
			wantIgnore:       false,
			wantErr:          false,
		},
		{
			name:       "devfile.yaml with invalid deploy.yaml reference",
			url:        "https://raw.githubusercontent.com/yangcao77/go-basic-no-deploy-file/main/devfile.yaml",
			wantIgnore: false,
			wantErr:    true,
		},
		{
			name:       "devfile.yaml should be ignored if no kubernetes components defined",
			url:        "https://raw.githubusercontent.com/devfile/registry/main/stacks/java-springboot/1.2.0/devfile.yaml",
			wantIgnore: true,
			wantErr:    false,
		},
		{
			name:       "devfile.yaml should be ignored if no image components defined",
			url:        "https://raw.githubusercontent.com/yangcao77/spring-sample-no-image-comp/main/devfile.yaml",
			wantIgnore: true,
			wantErr:    false,
		},
		{
			name:       "devfile.yaml with no outerloop definition and missing command group",
			url:        "https://raw.githubusercontent.com/yangcao77/missing-cmd-group/main/devfile.yaml",
			wantIgnore: true,
			wantErr:    false,
		},
		{
			name:       "should error out with multiple kubernetes components but no deploy command",
			url:        "https://raw.githubusercontent.com/yangcao77/spring-multi-kubecomps-no-deploycmd/main/devfile.yaml",
			wantIgnore: false,
			wantErr:    true,
		},
		{
			name:       "should error out with multiple image components but no apply command",
			url:        "https://raw.githubusercontent.com/yangcao77/spring-multi-imagecomps-no-applycmd/main/devfile.yaml",
			wantIgnore: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldIgnoreDevfile, devfileBytes, err := ValidateDevfile(logger, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestValidateDevfile() unexpected error: %v", err)
			}
			if !tt.wantErr {
				if shouldIgnoreDevfile != tt.wantIgnore {
					t.Errorf("TestValidateDevfile() wantIgnore is %v, got %v", tt.wantIgnore, shouldIgnoreDevfile)
				}
				if !tt.wantIgnore && !reflect.DeepEqual(devfileBytes, tt.wantDevfileBytes) {
					t.Errorf("devfile content did not match, got %v, wanted %v", string(devfileBytes), string(tt.wantDevfileBytes))
				}
			}

		})
	}
}

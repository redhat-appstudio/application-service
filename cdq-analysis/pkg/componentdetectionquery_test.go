//
// Copyright 2023 Red Hat, Inc.
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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCloneAndAnalyze(t *testing.T) {

	ctx := context.TODO()
	clientset := fake.NewSimpleClientset()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{})))
	log := ctrl.Log.WithName("TestCloneAndAnalyze")

	k8sClient := K8sInfoClient{
		Ctx:          ctx,
		Clientset:    clientset,
		Log:          log,
		CreateK8sJob: false,
	}

	compName := "testComponent"
	namespaceName := "testNamespace"
	springSampleURL := "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	springNoDevfileURL := "https://github.com/yangcao77/devfile-sample-java-springboot-basic-no-devfile"
	privateRepoURL := "https://github.com/johnmcollier/private-repo-test"
	multiComponentRepoURL := "https://github.com/maysunfaisal/multi-components-dockerfile"

	failedToCloneRepoErr := "failed to clone the repo.*"

	springDevfileContext := `
	schemaVersion: 2.2.0
	metadata:
	  name: java-springboot
	  version: 1.2.1
	  projectType: springboot
	  provider: Red Hat
	  language: Java
	`

	pythonDevfileContext := `
	schemaVersion: 2.2.0
	metadata:
	name: python
	version: 1.0.1
	projectType: Python
	provider: Red Hat
	language: Python
	`

	nodeJSDevfileContext := `
	schemaVersion: 2.2.0
	metadata:
	name: nodejs
	version: 2.1.1
	projectType: Node.js
	provider: Red Hat
	language: JavaScript
	`

	tests := []struct {
		testCase                 string
		context                  string
		devfilePath              string
		URL                      string
		Revision                 string
		DevfileRegistryURL       string
		gitToken                 string
		isDevfilePresent         bool
		isDockerfilePresent      bool
		wantErr                  string
		wantDevfilesMap          map[string][]byte
		wantDevfilesURLMap       map[string]string
		wantDockerfileContextMap map[string]string
		wantComponentsPortMap    map[string][]int
	}{
		{
			testCase:            "repo with devfile - should successfully detect spring component",
			URL:                 springSampleURL,
			DevfileRegistryURL:  DevfileRegistryEndpoint,
			devfilePath:         "devfile.yaml",
			isDevfilePresent:    true,
			isDockerfilePresent: false,
			wantDevfilesMap: map[string][]byte{
				"./": []byte(springDevfileContext),
			},
			wantDevfilesURLMap: map[string]string{
				"./": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
			},
			wantDockerfileContextMap: map[string]string{},
			wantComponentsPortMap:    map[string][]int{},
		},
		{
			testCase:            "repo without devfile and dockerfile - should successfully detect spring component",
			URL:                 springNoDevfileURL,
			DevfileRegistryURL:  DevfileRegistryEndpoint,
			isDevfilePresent:    false,
			isDockerfilePresent: false,
			wantDevfilesMap: map[string][]byte{
				"./": []byte(springDevfileContext),
			},
			wantDevfilesURLMap: map[string]string{
				"./": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
			},
			wantDockerfileContextMap: map[string]string{
				"./": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
			},
			wantComponentsPortMap: map[string][]int{
				"./": {8081},
			},
		},
		{
			testCase:                 "private repo - should error out with no token provided",
			URL:                      privateRepoURL,
			DevfileRegistryURL:       DevfileRegistryEndpoint,
			isDevfilePresent:         false,
			isDockerfilePresent:      false,
			wantDevfilesMap:          map[string][]byte{},
			wantDevfilesURLMap:       map[string]string{},
			wantDockerfileContextMap: map[string]string{},
			wantComponentsPortMap:    map[string][]int{},
			wantErr:                  failedToCloneRepoErr,
		},
		{
			testCase:                 "private repo - should error out with invalid token provided",
			URL:                      privateRepoURL,
			DevfileRegistryURL:       DevfileRegistryEndpoint,
			isDevfilePresent:         false,
			isDockerfilePresent:      false,
			gitToken:                 "fakeToken",
			wantDevfilesMap:          map[string][]byte{},
			wantDevfilesURLMap:       map[string]string{},
			wantDockerfileContextMap: map[string]string{},
			wantComponentsPortMap:    map[string][]int{},
			wantErr:                  failedToCloneRepoErr,
		},
		{
			testCase:            "should successfully detect multi-component with dockerfile present",
			URL:                 multiComponentRepoURL,
			DevfileRegistryURL:  DevfileRegistryEndpoint,
			isDevfilePresent:    false,
			isDockerfilePresent: false,
			wantDevfilesMap: map[string][]byte{
				"devfile-sample-java-springboot-basic": []byte(springDevfileContext),
				"devfile-sample-nodejs-basic":          []byte(nodeJSDevfileContext),
				"devfile-sample-python-basic":          []byte(pythonDevfileContext),
				"python-src-none":                      []byte(pythonDevfileContext),
			},
			wantDevfilesURLMap: map[string]string{
				"devfile-sample-java-springboot-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
				"devfile-sample-nodejs-basic":          "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml",
				"devfile-sample-python-basic":          "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-python-basic/.devfile.yaml",
				"python-src-none":                      "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml",
			},
			wantDockerfileContextMap: map[string]string{
				"devfile-sample-nodejs-basic": "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile",
				"devfile-sample-python-basic": "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-python-basic/Dockerfile",
				"python-src-docker":           "Dockerfile",
				"python-src-none":             "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile",
			},
			wantComponentsPortMap: map[string][]int{
				"devfile-sample-nodejs-basic": {3000},
			},
		},
		{
			testCase:            "should successfully detect single component when context is provided",
			context:             "devfile-sample-nodejs-basic",
			URL:                 multiComponentRepoURL,
			DevfileRegistryURL:  DevfileRegistryEndpoint,
			devfilePath:         "devfile.yaml",
			isDevfilePresent:    true,
			isDockerfilePresent: false,
			wantDevfilesMap: map[string][]byte{
				"devfile-sample-nodejs-basic": []byte(nodeJSDevfileContext),
			},
			wantDevfilesURLMap: map[string]string{
				"devfile-sample-nodejs-basic": "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml",
			},
			wantComponentsPortMap: map[string][]int{
				"devfile-sample-nodejs-basic": {3000},
			},
			wantDockerfileContextMap: map[string]string{
				"devfile-sample-nodejs-basic": "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testCase, func(t *testing.T) {
			devfilesMap, devfilesURLMap, dockerfileContextMap, componentsPortMap, err := CloneAndAnalyze(k8sClient, tt.gitToken, namespaceName, compName, tt.context, tt.devfilePath, "", tt.URL, tt.Revision, tt.DevfileRegistryURL, tt.isDevfilePresent, tt.isDockerfilePresent)
			if (err != nil) != (tt.wantErr != "") {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				// do not bother to check for the content for cdq testing
				// check if correct number of devfiles stored in the map
				// also check if correct context has been detected
				if devfilesMap != nil {
					if len(devfilesMap) != len(tt.wantDevfilesMap) {
						t.Errorf("Expected devfilesMap lenth: %+v, Got: %+v, devfileMap is %+v", len(tt.wantDevfilesMap), len(devfilesMap), devfilesMap)
					} else {
						for key := range tt.wantDevfilesMap {
							if _, ok := devfilesMap[key]; !ok {
								t.Errorf("Expected devfilesMap contains context: %+v, devfileMap is %+v", key, devfilesMap)
							}
						}
					}
				}
				if !reflect.DeepEqual(devfilesURLMap, tt.wantDevfilesURLMap) {
					t.Errorf("Expected devfilesURLMap: %+v, Got: %+v", tt.wantDevfilesURLMap, devfilesURLMap)
				}
				if !reflect.DeepEqual(dockerfileContextMap, tt.wantDockerfileContextMap) {
					t.Errorf("Expected dockerfileContextMap: %+v, Got: %+v", tt.wantDockerfileContextMap, dockerfileContextMap)
				}
				if !reflect.DeepEqual(componentsPortMap, tt.wantComponentsPortMap) {
					t.Errorf("Expected componentsPortMap: %+v, Got: %+v", tt.wantComponentsPortMap, componentsPortMap)
				}
			} else if err != nil {
				assert.Regexp(t, tt.wantErr, err.Error(), "Error message should match")
			}
			clientset.CoreV1().ConfigMaps(namespaceName).Delete(k8sClient.Ctx, compName, metav1.DeleteOptions{})
		})
	}
}

func TestSendBackDetectionResult(t *testing.T) {

	ctx := context.TODO()
	clientset := fake.NewSimpleClientset()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{})))
	log := ctrl.Log.WithName("TestSendBackDetectionResult")

	k8sClient := K8sInfoClient{
		Ctx:          ctx,
		Clientset:    clientset,
		Log:          log,
		CreateK8sJob: true,
	}

	compName := "testComponent"
	namespaceName := "testNamespace"

	springDevfileContext := `
schemaVersion: 2.2.0
metadata:
  name: java-springboot
  version: 1.2.1
  projectType: springboot
  provider: Red Hat
  language: Java
`
	devfilesMap := map[string][]byte{
		"./": []byte(springDevfileContext),
	}
	devfilesURLMap := map[string]string{
		"./": "https://registry.devfile.io/devfiles/java-springboot-basic",
	}
	dockerfileContextMap := map[string]string{
		"./": "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
	}

	configMapBinaryData := make(map[string][]byte)
	devfilesMapbytes, _ := json.Marshal(devfilesMap)
	configMapBinaryData["devfilesMap"] = devfilesMapbytes
	devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
	configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
	dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)
	configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes

	internalErrBinaryData := make(map[string][]byte)
	internalErr := fmt.Errorf("dummy internal error")
	internalErrMap := make(map[string]string)
	internalErrMap["InternalError"] = fmt.Sprintf("%v", internalErr)
	errorMapbytes, _ := json.Marshal(internalErrMap)
	internalErrBinaryData["errorMap"] = errorMapbytes

	devfileNotFoundBinaryData := make(map[string][]byte)
	devfileNotFoundErr := NoDevfileFound{"dummy location", fmt.Errorf("dummy NoDevfileFound error")}
	devfileNotFoundErrMap := make(map[string]string)
	devfileNotFoundErrMap["NoDevfileFound"] = fmt.Sprintf("%v", &devfileNotFoundErr)
	devfileNotFoundErrorMapbytes, _ := json.Marshal(devfileNotFoundErrMap)
	devfileNotFoundBinaryData["errorMap"] = devfileNotFoundErrorMapbytes

	dockerfileNotFoundBinaryData := make(map[string][]byte)
	dockerfileNotFoundErr := NoDockerfileFound{"dummy location", fmt.Errorf("dummy NoDockerfileFound error")}
	dockerfileNotFoundErrMap := make(map[string]string)
	dockerfileNotFoundErrMap["NoDockerfileFound"] = fmt.Sprintf("%v", &dockerfileNotFoundErr)
	dockerfileNotFoundErrorMapbytes, _ := json.Marshal(dockerfileNotFoundErrMap)
	dockerfileNotFoundBinaryData["errorMap"] = dockerfileNotFoundErrorMapbytes

	tests := []struct {
		testCase             string
		devfilesMap          map[string][]byte
		devfilesURLMap       map[string]string
		dockerfileContextMap map[string]string
		componentPortsMap    map[string][]int
		binaryData           map[string][]byte
		err                  error
	}{
		{
			testCase:             "without error",
			devfilesMap:          devfilesMap,
			devfilesURLMap:       devfilesURLMap,
			dockerfileContextMap: dockerfileContextMap,
			binaryData:           configMapBinaryData,
		},
		{
			testCase:   "with internal error",
			binaryData: internalErrBinaryData,
			err:        internalErr,
		},
		{
			testCase:   "with NoDevfileFound error",
			binaryData: devfileNotFoundBinaryData,
			err:        &devfileNotFoundErr,
		},
		{
			testCase:   "with NoDockerfileFound error",
			binaryData: dockerfileNotFoundBinaryData,
			err:        &dockerfileNotFoundErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testCase, func(t *testing.T) {
			k8sClient.SendBackDetectionResult(tt.devfilesMap, tt.devfilesURLMap, tt.dockerfileContextMap, tt.componentPortsMap, compName, namespaceName, tt.err)
			configMap, err := clientset.CoreV1().ConfigMaps(namespaceName).Get(k8sClient.Ctx, compName, metav1.GetOptions{})
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			}
			if !reflect.DeepEqual(tt.binaryData, configMap.BinaryData) {
				t.Errorf("Expected configmap with binaryData: %+v, Got: %+v", tt.binaryData, configMap.BinaryData)
			}
			clientset.CoreV1().ConfigMaps(namespaceName).Delete(k8sClient.Ctx, compName, metav1.DeleteOptions{})
		})
	}
}

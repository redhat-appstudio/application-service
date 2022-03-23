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
	"os"
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/util"
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

func TestConvertApplicationToDevfile(t *testing.T) {
	additionalAttributes := attributes.Attributes{}.PutString("appModelRepository.branch", "testbranch").PutString("gitOpsRepository.branch", "testbranch").PutString("appModelRepository.context", "test/context").PutString("gitOpsRepository.context", "test/context")

	tests := []struct {
		name         string
		hasApp       appstudiov1alpha1.Application
		appModelRepo string
		gitOpsRepo   string
		wantDevfile  *v2.DevfileV2
	}{
		{
			name: "Simple HASApp CR",
			hasApp: appstudiov1alpha1.Application{
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: "Petclinic",
				},
			},
			appModelRepo: "https://github.com/testorg/petclinic-app",
			gitOpsRepo:   "https://github.com/testorg/petclinic-gitops",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion210),
						Metadata: devfile.DevfileMetadata{
							Name:       "Petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
		},
		{
			name: "HASApp CR with branch and context fields set",
			hasApp: appstudiov1alpha1.Application{
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: "Petclinic",
					AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
						Branch:  "testbranch",
						Context: "test/context",
					},
					GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
						Branch:  "testbranch",
						Context: "test/context",
					},
				},
			},
			appModelRepo: "https://github.com/testorg/petclinic-app",
			gitOpsRepo:   "https://github.com/testorg/petclinic-gitops",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion210),
						Metadata: devfile.DevfileMetadata{
							Name:       "Petclinic",
							Attributes: additionalAttributes.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			convertedDevfile, err := ConvertApplicationToDevfile(tt.hasApp, tt.gitOpsRepo, tt.appModelRepo)
			if err != nil {
				t.Errorf("TestConvertApplicationToDevfile() unexpected error: %v", err)
			} else if !reflect.DeepEqual(convertedDevfile, tt.wantDevfile) {
				t.Errorf("TestConvertApplicationToDevfile() error: expected %v got %v", tt.wantDevfile, convertedDevfile)
			}
		})
	}
}

func TestDownloadDevfile(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name: "Curl devfile.yaml",
			url:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name: "Curl .devfile.yaml",
			url:  "https://raw.githubusercontent.com/maysunfaisal/hiddendevfile/main",
		},
		{
			name: "Curl .devfile/devfile.yaml",
			url:  "https://raw.githubusercontent.com/maysunfaisal/hiddendirdevfile/main",
		},
		{
			name: "Curl .devfile/.devfile.yaml",
			url:  "https://raw.githubusercontent.com/maysunfaisal/hiddendirhiddendevfile/main",
		},
		{
			name:    "Cannot curl for a devfile",
			url:     "https://github.com/octocat/Hello-World",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := DownloadDevfile(tt.url)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil && contents == nil {
				t.Errorf("unable to read body")
			}
		})
	}
}

func TestReadDevfilesFromRepo(t *testing.T) {

	var mockClient MockAlizerClient

	tests := []struct {
		name                   string
		clonePath              string
		depth                  int
		repo                   string
		token                  string
		wantErr                bool
		expectedDevfileContext []string
	}{
		{
			name:      "Should return 0 devfiles as this is not a multi comp devfile",
			clonePath: "/tmp/testclone",
			depth:     1,
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			wantErr:   true,
		},
		{
			name:                   "Should return 1 devfiles as this is a multi comp devfile",
			clonePath:              "/tmp/testclone",
			depth:                  1,
			repo:                   "https://github.com/maysunfaisal/multi-components-deep",
			expectedDevfileContext: []string{"devfile-sample-java-springboot-basic", "python"},
		},
		// TODO - maysunfaisal
		// Commenting out this test case, we hard code our depth to 1 for CDQ
		// But there seems to a gap in the logic if we extend past depth 1 and discovering devfile logic
		// Revisit post M4

		// {
		// 	name:                   "Should return 2 devfiles as this is a multi comp devfile",
		// 	clonePath:              "/tmp/testclone",
		// 	depth:                  2,
		// 	repo:                   "https://github.com/maysunfaisal/multi-components-deep",
		// 	expectedDevfileContext: []string{"devfile-sample-java-springboot-basic", "python/devfile-sample-python-basic"},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := util.CloneRepo(tt.clonePath, tt.repo, tt.token)
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				devfileMap, _, err := ReadDevfilesFromRepo(mockClient, tt.clonePath, tt.depth, DevfileStageRegistryEndpoint)
				if tt.wantErr && (err == nil) {
					t.Error("wanted error but got nil")
				} else if !tt.wantErr && err != nil {
					t.Errorf("got unexpected error %v", err)
				} else {
					for actualContext := range devfileMap {
						matched := false
						for _, context := range tt.expectedDevfileContext {
							if context == actualContext {
								matched = true
							}
						}

						if !matched {
							t.Errorf("found devfile at context %v but expected none", actualContext)
						}
					}
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

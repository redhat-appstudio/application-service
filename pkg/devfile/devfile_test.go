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
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/stretchr/testify/assert"
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
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app").PutString("gitOpsRepository.context", "/").PutString("appModelRepository.context", "/"),
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

func TestConvertImageComponentToDevfile(t *testing.T) {
	//devfileAttributes := attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
	tests := []struct {
		name        string
		comp        appstudiov1alpha1.Component
		wantDevfile *v2.DevfileV2
	}{
		{
			name: "Simple Component CR",
			comp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "Petclinic",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							ImageSource: &appstudiov1alpha1.ImageSource{
								ContainerImage: "quay.io/test/someimage:latest",
							},
						},
					},
				},
			},
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion210),
						Metadata: devfile.DevfileMetadata{
							Name: "Petclinic",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "container",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											Container: v1alpha2.Container{
												Image: "quay.io/test/someimage:latest",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			convertedDevfile, err := ConvertImageComponentToDevfile(tt.comp)
			if err != nil {
				t.Errorf("TestConvertImageComponentToDevfile() unexpected error: %v", err)
			} else if !reflect.DeepEqual(convertedDevfile, tt.wantDevfile) {
				t.Errorf("TestConvertImageComponentToDevfile() error: expected %v got %v", tt.wantDevfile, convertedDevfile)
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

func TestCreateDevfileForDockerfileBuild(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		context string
		wantErr bool
	}{
		{
			name:    "Set Dockerfile Uri and Context",
			uri:     "dockerfile/uri",
			context: "context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDevfile, err := CreateDevfileForDockerfileBuild(tt.uri, tt.context)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				// Devfile Metadata
				metadata := gotDevfile.GetMetadata()
				assert.Equal(t, "dockerfile-component", metadata.Name, "Devfile metadata name should be equal")
				assert.Equal(t, "Basic Devfile for a Dockerfile Component", metadata.Description, "Devfile metadata description should be equal")

				// Container Component
				if containerComponents, err := gotDevfile.GetComponents(common.DevfileOptions{
					ComponentOptions: common.ComponentOptions{
						ComponentType: v1alpha2.ContainerComponentType,
					},
				}); err != nil {
					t.Errorf("unexpected error %v", err)
				} else if len(containerComponents) != 1 {
					t.Error("expected 1 container component")
				} else {
					assert.Equal(t, "container", containerComponents[0].Name, "component name should be equal")
					assert.Equal(t, "no-op", containerComponents[0].Container.Image, "container image should be equal")
				}

				// Image Component
				if imageComponents, err := gotDevfile.GetComponents(common.DevfileOptions{
					ComponentOptions: common.ComponentOptions{
						ComponentType: v1alpha2.ImageComponentType,
					},
				}); err != nil {
					t.Errorf("unexpected error %v", err)
					return
				} else if len(imageComponents) != 1 {
					t.Error("expected 1 image component")
				} else {
					assert.Equal(t, "dockerfile-build", imageComponents[0].Name, "component name should be equal")
					assert.NotNil(t, imageComponents[0].Image, "Image component should not be nil")
					assert.NotNil(t, imageComponents[0].Image.Dockerfile, "Dockerfile Image component should not be nil")
					assert.Equal(t, tt.uri, imageComponents[0].Image.Dockerfile.DockerfileSrc.Uri, "dockerfile uri should be equal")
					assert.Equal(t, tt.context, imageComponents[0].Image.Dockerfile.Dockerfile.BuildContext, "dockerfile context should be equal")
				}

				// Apply Command
				if applyCommands, err := gotDevfile.GetCommands(common.DevfileOptions{
					CommandOptions: common.CommandOptions{
						CommandType: v1alpha2.ApplyCommandType,
					},
				}); err != nil {
					t.Errorf("unexpected error %v", err)
					return
				} else if len(applyCommands) != 1 {
					t.Error("expected 1 apply command")
				} else {
					assert.Equal(t, "build-image", applyCommands[0].Id, "command id should be equal")
					assert.NotNil(t, applyCommands[0].Apply, "Apply command should not be nil")
					assert.Equal(t, "dockerfile-build", applyCommands[0].Apply.Component, "command component reference should be equal")
				}
			}
		})
	}
}

func TestDownloadDevfileAndDockerfile(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "Curl devfile.yaml and dockerfile",
			url:  "https://raw.githubusercontent.com/maysunfaisal/devfile-sample-python-samelevel/main",
			want: true,
		},
		{
			name: "Cannot curl for a devfile nor a dockerfile",
			url:  "https://github.com/octocat/Hello-World",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfile, dockerfile := DownloadDevfileAndDockerfile(tt.url)
			if tt.want != (len(devfile) > 0 && len(dockerfile) > 0) {
				t.Errorf("devfile and a dockerfile wanted: %v but got devfile: %v dockerfile: %v", tt.want, len(devfile) > 0, len(dockerfile) > 0)
			}
		})
	}
}

func TestScanRepo(t *testing.T) {

	var mockClient MockAlizerClient

	tests := []struct {
		name                      string
		clonePath                 string
		depth                     int
		repo                      string
		token                     string
		wantErr                   bool
		expectedDevfileContext    []string
		expectedDevfileURLContext []string
		expectedDockerfileContext []string
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
		// {
		// 	name:                      "Should return x devfiles as this is a multi comp devfile",
		// 	clonePath:                 "/tmp/testclone",
		// 	depth:                     1,
		// 	repo:                      "https://github.com/maysunfaisal/multi-components-dockerfile",
		// 	expectedDevfileContext:    []string{"devfile-sample-java-springboot-basic", "devfile-sample-nodejs-basic", "devfile-sample-python-basic", "python-src-none"},
		// 	expectedDevfileURLContext: []string{"python-src-none"},
		// 	expectedDockerfileContext: []string{"python-src-docker"},
		// },
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
				devfileMap, _, _, err := ScanRepo(nil, mockClient, tt.clonePath, tt.depth, DevfileStageRegistryEndpoint)
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

					// for context, uri := range devfileURLMap {
					// 	t.Logf("devfileURLMAP context %v, uri %v", context, uri)
					// }

					// for context, uri := range dockerfileMap {
					// 	t.Logf("dockerfileMap context %v, uri %v", context, uri)
					// }
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

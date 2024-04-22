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

package devfile

import (
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	routev1 "github.com/openshift/api/route/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertApplicationToDevfile(t *testing.T) {
	tests := []struct {
		name        string
		hasApp      appstudiov1alpha1.Application
		wantDevfile *v2.DevfileV2
	}{
		{
			name: "Simple HASApp CR",
			hasApp: appstudiov1alpha1.Application{
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: "Petclinic",
				},
			},
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "Petclinic",
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
				},
			},
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "Petclinic",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			convertedDevfile, err := ConvertApplicationToDevfile(tt.hasApp)
			if err != nil {
				t.Errorf("TestConvertApplicationToDevfile() unexpected error: %v", err)
			} else if !reflect.DeepEqual(convertedDevfile, tt.wantDevfile) {
				t.Errorf("TestConvertApplicationToDevfile() error: expected %v got %v", tt.wantDevfile, convertedDevfile)
			}
		})
	}
}

func TestConvertImageComponentToDevfile(t *testing.T) {

	compName := "component"
	applicationName := "application"
	image := "image"

	deploymentTemplate := GenerateDeploymentTemplate(compName, applicationName, image)
	deploymentTemplateBytes, err := yaml.Marshal(deploymentTemplate)
	if err != nil {
		t.Errorf("TestConvertImageComponentToDevfile() unexpected error: %v", err)
		return
	}

	tests := []struct {
		name        string
		comp        appstudiov1alpha1.Component
		wantDevfile *v2.DevfileV2
	}{
		{
			name: "Simple Component CR",
			comp: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: compName,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  compName,
					ContainerImage: image,
					Application:    applicationName,
				},
			},
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: compName,
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "kubernetes-deploy",
									ComponentUnion: v1alpha2.ComponentUnion{
										Kubernetes: &v1alpha2.KubernetesComponent{
											K8sLikeComponent: v1alpha2.K8sLikeComponent{
												K8sLikeComponentLocation: v1alpha2.K8sLikeComponentLocation{
													Inlined: string(deploymentTemplateBytes),
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

func TestFindAndDownloadDevfile(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		wantDevfileContext string
		wantErr            bool
	}{
		{
			name:               "Curl devfile.yaml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case1",
			wantDevfileContext: "devfile.yaml",
		},
		{
			name:               "Curl .devfile.yaml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case2",
			wantDevfileContext: ".devfile.yaml",
		},
		{
			name:               "Curl devfile.yml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case3",
			wantDevfileContext: "devfile.yml",
		},
		{
			name:               "Curl .devfile.yml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case4",
			wantDevfileContext: ".devfile.yml",
		},
		{
			name:               "Curl .devfile/devfile.yaml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case5",
			wantDevfileContext: ".devfile/devfile.yaml",
		},
		{
			name:               "Curl .devfile/.devfile.yaml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case6",
			wantDevfileContext: ".devfile/.devfile.yaml",
		},
		{
			name:               "Curl .devfile/devfile.yml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case7",
			wantDevfileContext: ".devfile/devfile.yml",
		},
		{
			name:               "Curl .devfile/.devfile.yml",
			url:                "https://raw.githubusercontent.com/devfile-resources/devfile-priority/main/case8",
			wantDevfileContext: ".devfile/.devfile.yml",
		},
		{
			name:    "Cannot curl for a devfile",
			url:     "https://github.com/octocat/Hello-World",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, devfileContext, err := FindAndDownloadDevfile(tt.url, "")
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

func TestFindAndDownloadDockerfile(t *testing.T) {
	tests := []struct {
		name                  string
		url                   string
		wantDockerfileContext string
		wantErr               bool
	}{
		{
			name:                  "Curl Dockerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case1",
			wantDockerfileContext: "Dockerfile",
		},
		{
			name:                  "Curl docker/Dockerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case2",
			wantDockerfileContext: "docker/Dockerfile",
		},
		{
			name:                  "Curl .docker/Dockerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case3",
			wantDockerfileContext: ".docker/Dockerfile",
		},
		{
			name:                  "Curl build/Dockerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case4",
			wantDockerfileContext: "build/Dockerfile",
		},
		{
			name:                  "Curl Containerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case5",
			wantDockerfileContext: "Containerfile",
		},
		{
			name:                  "Curl docker/Containerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case6",
			wantDockerfileContext: "docker/Containerfile",
		},
		{
			name:                  "Curl .docker/Containerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case7",
			wantDockerfileContext: ".docker/Containerfile",
		},
		{
			name:                  "Curl build/Containerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case8",
			wantDockerfileContext: "build/Containerfile",
		},
		{
			name:                  "Curl dockerfile",
			url:                   "https://raw.githubusercontent.com/devfile-resources/dockerfile-priority/main/case9",
			wantDockerfileContext: "dockerfile",
		},
		{
			name:    "Cannot curl for a Dockerfile or a Containerfile",
			url:     "https://github.com/octocat/Hello-World",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, dockerfileContext, err := FindAndDownloadDockerfile(tt.url, "")
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil && contents == nil {
				t.Errorf("unable to read body")
			} else if err == nil && (dockerfileContext != tt.wantDockerfileContext) {
				t.Errorf("Dockerfile context did not match, got %v, wanted %v", dockerfileContext, tt.wantDockerfileContext)
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
			gotDevfile, err := CreateDevfileForDockerfileBuild(tt.uri, tt.context, "", "")
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				// Devfile Metadata
				metadata := gotDevfile.GetMetadata()
				assert.Equal(t, "dockerfile-component", metadata.Name, "Devfile metadata name should be equal")
				assert.Equal(t, "Basic Devfile for a Dockerfile Component", metadata.Description, "Devfile metadata description should be equal")

				// Kubernetes Component
				if kubernetesComponents, err := gotDevfile.GetComponents(common.DevfileOptions{
					ComponentOptions: common.ComponentOptions{
						ComponentType: v1alpha2.KubernetesComponentType,
					},
				}); err != nil {
					t.Errorf("unexpected error %v", err)
				} else if len(kubernetesComponents) != 1 {
					t.Error("expected 1 Kubernetes component")
				} else {
					assert.Equal(t, "kubernetes-deploy", kubernetesComponents[0].Name, "component name should be equal")
					assert.Contains(t, kubernetesComponents[0].Kubernetes.Inlined, "Deployment", "the inlined content should contain deployment")
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
					assert.Equal(t, tt.uri, imageComponents[0].Image.Dockerfile.DockerfileSrc.Uri, "Dockerfile uri should be equal")
					assert.Equal(t, tt.context, imageComponents[0].Image.Dockerfile.Dockerfile.BuildContext, "Dockerfile context should be equal")
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

func TestGetRouteFromEndpoint(t *testing.T) {

	var (
		name        = "route1"
		serviceName = "service1"
		path        = ""
		port        = "1234"
		secure      = true
		annotations = map[string]string{
			"key1": "value1",
		}
	)
	t.Run(name, func(t *testing.T) {
		actualRoute := GetRouteFromEndpoint(name, serviceName, port, path, secure, annotations)
		assert.Equal(t, "Route", actualRoute.Kind, "Kind did not match")
		assert.Equal(t, "route.openshift.io/v1", actualRoute.APIVersion, "APIVersion did not match")
		assert.Equal(t, name, actualRoute.Name, "Route name did not match")
		assert.Equal(t, "/", actualRoute.Spec.Path, "Route path did not match")
		assert.NotNil(t, actualRoute.Spec.Port, "Route Port should not be nil")
		assert.Equal(t, intstr.FromString(port), actualRoute.Spec.Port.TargetPort, "Route port did not match")
		assert.NotNil(t, actualRoute.Spec.TLS, "Route TLS should not be nil")
		assert.Equal(t, routev1.TLSTerminationEdge, actualRoute.Spec.TLS.Termination, "Route port did not match")
		actualRouteAnnotations := actualRoute.GetAnnotations()
		assert.NotEmpty(t, actualRouteAnnotations, "Route annotations should not be empty")
		assert.Equal(t, "value1", actualRouteAnnotations["key1"], "Route annotation did not match")
	})
}

func TestGenerateDeploymentTemplate(t *testing.T) {

	var (
		name        = "deploy1"
		application = "application1"
		image       = "image1"
	)
	t.Run(name, func(t *testing.T) {
		actualDeployment := GenerateDeploymentTemplate(name, application, image)
		assert.Equal(t, "Deployment", actualDeployment.Kind, "Kind did not match")
		assert.Equal(t, name, actualDeployment.Name, "Name did not match")
		assert.Equal(t, generateK8sLabels(name, application), actualDeployment.Labels, "Labels did not match")
		assert.NotNil(t, actualDeployment.Spec.Selector, "Selector can not be nil")
		assert.Equal(t, getMatchLabel(name), actualDeployment.Spec.Selector.MatchLabels, "Match Labels did not match")
		assert.Equal(t, getMatchLabel(name), actualDeployment.Spec.Template.Labels, "Match Labels did not match")
		assert.Equal(t, 1, len(actualDeployment.Spec.Template.Spec.Containers), "Should have only 1 container")
		assert.Equal(t, image, actualDeployment.Spec.Template.Spec.Containers[0].Image, "Container Image did not match")
	})
}

func TestUpdateLocalDockerfileURItoAbsolute(t *testing.T) {
	tests := []struct {
		name          string
		devfile       *v2.DevfileV2
		dockerfileURL string
		wantDevfile   *v2.DevfileV2
		wantErr       bool
	}{
		{
			name: "devfile.yaml with local Dockerfile URI references",
			devfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "SomeDevfile",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "image-build",
									ComponentUnion: v1alpha2.ComponentUnion{
										Image: &v1alpha2.ImageComponent{
											Image: v1alpha2.Image{
												ImageName: "component-image",
												ImageUnion: v1alpha2.ImageUnion{
													Dockerfile: &v1alpha2.DockerfileImage{
														DockerfileSrc: v1alpha2.DockerfileSrc{
															Uri: "./Dockerfile",
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
				},
			},
			dockerfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "SomeDevfile",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "image-build",
									ComponentUnion: v1alpha2.ComponentUnion{
										Image: &v1alpha2.ImageComponent{
											Image: v1alpha2.Image{
												ImageName: "component-image",
												ImageUnion: v1alpha2.ImageUnion{
													Dockerfile: &v1alpha2.DockerfileImage{
														DockerfileSrc: v1alpha2.DockerfileSrc{
															Uri: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
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
				},
			},
			wantErr: false,
		},
		{
			name: "devfile.yaml with local Dockerfile URI reference, and multiple other components",
			devfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "SomeDevfile",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "other-components",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											BaseComponent: v1alpha2.BaseComponent{},
										},
									},
								},
								{
									Name: "image-build",
									ComponentUnion: v1alpha2.ComponentUnion{
										Image: &v1alpha2.ImageComponent{
											Image: v1alpha2.Image{
												ImageName: "component-image",
												ImageUnion: v1alpha2.ImageUnion{
													Dockerfile: &v1alpha2.DockerfileImage{
														DockerfileSrc: v1alpha2.DockerfileSrc{
															Uri: "./Dockerfile",
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
				},
			},
			dockerfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "SomeDevfile",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "other-components",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											BaseComponent: v1alpha2.BaseComponent{},
										},
									},
								},
								{
									Name: "image-build",
									ComponentUnion: v1alpha2.ComponentUnion{
										Image: &v1alpha2.ImageComponent{
											Image: v1alpha2.Image{
												ImageName: "component-image",
												ImageUnion: v1alpha2.ImageUnion{
													Dockerfile: &v1alpha2.DockerfileImage{
														DockerfileSrc: v1alpha2.DockerfileSrc{
															Uri: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
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
				},
			},
			wantErr: false,
		},
		{
			name: "devfile.yaml with no local Dockerfile URI reference",
			devfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "SomeDevfile",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "other-components",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											BaseComponent: v1alpha2.BaseComponent{},
										},
									},
								},
								{
									Name: "another-component",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											BaseComponent: v1alpha2.BaseComponent{},
										},
									},
								},
							},
						},
					},
				},
			},
			dockerfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
			wantDevfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfile.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name: "SomeDevfile",
						},
					},
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									Name: "other-components",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											BaseComponent: v1alpha2.BaseComponent{},
										},
									},
								},
								{
									Name: "another-component",
									ComponentUnion: v1alpha2.ComponentUnion{
										Container: &v1alpha2.ContainerComponent{
											BaseComponent: v1alpha2.BaseComponent{},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "devfile.yaml with invalid components, should return err",
			devfile: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: []v1alpha2.Component{
								{
									ComponentUnion: v1alpha2.ComponentUnion{
										ComponentType: "bad-component",
									},
								},
							},
						},
					},
				},
			},
			dockerfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfile, err := UpdateLocalDockerfileURItoAbsolute(tt.devfile, tt.dockerfileURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestUpdateLocalDockerfileURItoAbsolute() unexpected error: %v", err)
			}

			if !tt.wantErr && !reflect.DeepEqual(devfile, tt.wantDevfile) {
				t.Errorf("devfile content did not match, got %v, wanted %v", devfile, tt.wantDevfile)
			}
		})
	}
}

func TestGetIngressFromEndpoint(t *testing.T) {

	componentName := "test-component"

	implementationSpecific := networkingv1.PathTypeImplementationSpecific

	tests := []struct {
		name        string
		ingressName string
		serviceName string
		port        string
		path        string
		hostname    string
		annotations map[string]string
		wantErr     bool
		wantIngress networkingv1.Ingress
	}{
		{
			name:        "Get simple ingress",
			ingressName: componentName,
			serviceName: componentName,
			port:        "5000",
			path:        "",
			hostname:    componentName + ".example.com",
			annotations: map[string]string{
				"test": "yes",
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: componentName,
					Annotations: map[string]string{
						"test": "yes",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: componentName,
													Port: networkingv1.ServiceBackendPort{
														Number: 5000,
													},
												},
											},
										},
									},
								},
							},
							Host: componentName + ".example.com",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generatedIngress, err := GetIngressFromEndpoint(tt.ingressName, tt.serviceName, tt.port, tt.path, false, tt.annotations, tt.hostname)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if !reflect.DeepEqual(tt.wantIngress, generatedIngress) {
				t.Errorf("Expected: %+v, \nGot: %+v", tt.wantIngress, generatedIngress)
			}
		})
	}
}

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
	"strings"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	parser "github.com/devfile/library/v2/pkg/devfile/parser"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfile.DevfileMetadata{
							Name:       "Petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app").PutString("gitOpsRepository.context", "./").PutString("appModelRepository.context", "/"),
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
						SchemaVersion: string(data.APISchemaVersion220),
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

func TestFindAndDownloadDockerfile(t *testing.T) {
	tests := []struct {
		name                  string
		url                   string
		wantDockerfileContext string
		wantErr               bool
	}{
		{
			name:                  "Curl Dockerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case1",
			wantDockerfileContext: "Dockerfile",
		},
		{
			name:                  "Curl docker/Dockerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case2",
			wantDockerfileContext: "docker/Dockerfile",
		},
		{
			name:                  "Curl .docker/Dockerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case3",
			wantDockerfileContext: ".docker/Dockerfile",
		},
		{
			name:                  "Curl build/Dockerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case4",
			wantDockerfileContext: "build/Dockerfile",
		},
		{
			name:                  "Curl Containerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case5",
			wantDockerfileContext: "Containerfile",
		},
		{
			name:                  "Curl docker/Containerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case6",
			wantDockerfileContext: "docker/Containerfile",
		},
		{
			name:                  "Curl .docker/Containerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case7",
			wantDockerfileContext: ".docker/Containerfile",
		},
		{
			name:                  "Curl build/Containerfile",
			url:                   "https://raw.githubusercontent.com/yangcao77/dockerfile-priority/main/case8",
			wantDockerfileContext: "build/Containerfile",
		},
		{
			name:    "Cannot curl for a Dockerfile or a Containerfile",
			url:     "https://github.com/octocat/Hello-World",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, dockerfileContext, err := FindAndDownloadDockerfile(tt.url)
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

func TestDownloadDevfileAndDockerfile(t *testing.T) {
	tests := []struct {
		name                  string
		url                   string
		wantDevfileContext    string
		wantDockerfileContext string
		want                  bool
	}{
		{
			name:                  "Curl devfile.yaml and Dockerfile",
			url:                   "https://raw.githubusercontent.com/maysunfaisal/devfile-sample-python-samelevel/main",
			wantDevfileContext:    ".devfile.yaml",
			wantDockerfileContext: "Dockerfile",
			want:                  true,
		},
		{
			name: "Cannot curl for a devfile nor a Dockerfile",
			url:  "https://github.com/octocat/Hello-World",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfile, devfileContext, dockerfile, dockerfileContext := DownloadDevfileAndDockerfile(tt.url)
			if tt.want != (len(devfile) > 0 && len(dockerfile) > 0) {
				t.Errorf("devfile and a Dockerfile wanted: %v but got devfile: %v Dockerfile: %v", tt.want, len(devfile) > 0, len(dockerfile) > 0)
			}

			if devfileContext != tt.wantDevfileContext {
				t.Errorf("devfile context did not match, got %v, wanted %v", devfileContext, tt.wantDevfileContext)
			}

			if dockerfileContext != tt.wantDockerfileContext {
				t.Errorf("Dockerfile context did not match, got %v, wanted %v", dockerfileContext, tt.wantDockerfileContext)
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

func TestGetResourceFromDevfile(t *testing.T) {

	weight := int32(100)

	kubernetesInlinedDevfile := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 5566
    deployment/containerENV:
    - name: FOO
      value: foo11
    - name: BAR
      value: bar11
    deployment/cpuLimit: "2"
    deployment/cpuRequest: 701m
    deployment/memoryLimit: 500Mi
    deployment/memoryRequest: 401Mi
    deployment/replicas: 5
    deployment/route: route111222
  kubernetes:
    deployByDefault: false
    endpoints:
    - name: http-8081
      path: /
      secure: false
      targetPort: 8081
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        labels:
          maysun: test
        name: deploy-sample
      spec:
        replicas: 1
        selector: {}
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
      status: {}
      ---
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: deploy-sample-2
      spec:
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOO
                value: foo1
              - name: BAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
      ---
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
        selector:
          app.kubernetes.io/instance: component-sample
      status:
        loadBalancer: {}
      ---
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: service-sample-2
      spec:
        ports:
        - port: 1111
          targetPort: 1111
        selector:
          app.kubernetes.io/instance: component-sample
      status:
        loadBalancer: {}
      ---
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: route-sample
      spec:
        host: route111
        port:
          targetPort: 1111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
      ---
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: route-sample-2
      spec:
        host: route111
        port:
          targetPort: 1111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
      ---
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ingress-sample
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          maysun: test
      spec:
        ingressClassName: nginx-example
        rules:
        - http:
            paths:
            - path: /testpath
              pathType: Prefix
              backend:
                service:
                  name: test
                  port:
                    number: 80
      ---
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ingress-sample-2
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          maysun: test
      spec:
        ingressClassName: nginx-example
        rules:
        - http:
            paths:
            - path: /testpath
              pathType: Prefix
              backend:
                service:
                  name: test
                  port:
                    number: 80
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: pvc-sample
        labels:
          maysun: test
      spec:
        accessModes:
          - ReadWriteOnce
        volumeMode: Filesystem
        resources:
          requests:
            storage: 8Gi
        storageClassName: slow
        selector:
          matchLabels:
            release: "stable"
          matchExpressions:
            - {key: environment, operator: In, values: [dev]}
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: pvc-sample-2
        labels:
          maysun: test
      spec:
        accessModes:
          - ReadWriteOnce
        volumeMode: Filesystem
        resources:
          requests:
            storage: 8Gi
        storageClassName: slow
        selector:
          matchLabels:
            release: "stable"
          matchExpressions:
            - {key: environment, operator: In, values: [dev]}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileIngress := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 5566
    deployment/containerENV:
    - name: FOO
      value: foo11
    - name: BAR
      value: bar11
    deployment/cpuLimit: "2"
    deployment/cpuRequest: 701m
    deployment/memoryLimit: 500Mi
    deployment/memoryRequest: 401Mi
    deployment/replicas: 5
    deployment/route: route111222
    deployment/storageLimit: 400Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ingress-sample
        labels:
          test: test
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          test: yes
      spec:
        rules:
        - host: "foo.bar.com"
          http:
            paths:
            - path: /testpath
              pathType: ImplementationSpecific
              backend:
                service:
                  name: test
                  port:
                    number: 80
      status: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileRoute := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 5566
    deployment/containerENV:
    - name: FOO
      value: foo11
    - name: BAR
      value: bar11
    deployment/cpuLimit: "2"
    deployment/cpuRequest: 701m
    deployment/memoryLimit: 500Mi
    deployment/memoryRequest: 401Mi
    deployment/replicas: 5
    deployment/route: route111222
    deployment/storageLimit: 400Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        name: route-sample-2
        labels:
          test: test
      spec:
        host: route111
        port:
          targetPort: 1111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileRouteNoTargetPort := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 5566
    deployment/containerENV:
    - name: FOO
      value: foo11
    - name: BAR
      value: bar11
    deployment/cpuLimit: "2"
    deployment/cpuRequest: 701m
    deployment/memoryLimit: 500Mi
    deployment/memoryRequest: 401Mi
    deployment/replicas: 5
    deployment/route: route111222
    deployment/storageLimit: 400Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        name: route-sample-2
        labels:
          test: test
      spec:
        host: route111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileSvc := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileDeploy := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 1111
    deployment/storageLimit: 401Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    endpoints:
    - name: http-8081
      path: /
      secure: false
      targetPort: 8081
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        name: deploy-sample
      spec:
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOOFOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileSeparatedKubeComps := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- apply:
    component: kubernetes-svc
  id: svck8s
- composite:
    commands:
    - build-image
    - deployk8s
    - svck8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 1111
    deployment/storageLimit: 401Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    endpoints:
    - name: http-8081
      path: /
      secure: false
      targetPort: 8081
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        name: deploy-sample
      spec:
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOOFOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
  name: kubernetes-deploy
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-svc
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileSeparatedKubeCompsRevHistory := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- apply:
    component: kubernetes-svc
  id: svck8s
- composite:
    commands:
    - build-image
    - deployk8s
    - svck8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 1111
    deployment/storageLimit: 401Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    endpoints:
    - name: http-8081
      path: /
      secure: false
      targetPort: 8081
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        name: deploy-sample
      spec:
        revisionHistoryLimit: 5
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOOFOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
  name: kubernetes-deploy
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-svc
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileRouteHostMissing := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 5566
  kubernetes:
    deployByDefault: false
    endpoints:
    - name: http-8081
      path: /
      secure: false
      targetPort: 8081
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: deploy-sample
      spec:
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOOFOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesWithoutInline := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- kubernetes:
    deployByDefault: false
    uri: ""
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadMemoryLimit := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/memoryLimit: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadStorageLimit := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/storageLimit: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadCPULimit := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/cpuLimit: "abc"
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadMemoryRequest := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/memoryRequest: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadStorageRequest := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/storageRequest: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadCPUrequest := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/cpuRequest: "abc"
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	noKubernetesCompDevfile := `
commands:
- apply:
    component: image-build
  id: build-image
- composite:
    commands:
    - build-image
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	multipleKubernetesCompsDevfile := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample2
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy2
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesCompsWithNoDeployCmdDevfile := `
commands:
- apply:
    component: image-build
  id: build-image
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	replica := int32(5)
	replicaUpdated := int32(1)
	revHistoryLimit := int32(0)
	setRevHistoryLimit := int32(5)

	host := "host.example.com"
	implementationSpecific := networkingv1.PathTypeImplementationSpecific

	tests := []struct {
		name          string
		devfileString string
		componentName string
		appName       string
		image         string
		hostname      string
		wantDeploy    appsv1.Deployment
		wantService   corev1.Service
		wantRoute     routev1.Route
		wantIngress   networkingv1.Ingress
		wantErr       bool
	}{
		{
			name:          "Simple devfile from Inline",
			devfileString: kubernetesInlinedDevfile,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replica,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOO",
											Value: "foo11",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
										{
											Name:  "BAR",
											Value: "bar11",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
										{
											ContainerPort: int32(5566),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("2"),
											corev1.ResourceMemory: resource.MustParse("500Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("701m"),
											corev1.ResourceMemory: resource.MustParse("401Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
						{
							Port:       int32(5566),
							TargetPort: intstr.FromInt(5566),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 5566,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Host: "route111222",
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with deployment and svc from separated kube components",
			devfileString: kubernetesInlinedDevfileSeparatedKubeComps,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with deployment and svc from separated kube components - with RevisionHistoryLimit set",
			devfileString: kubernetesInlinedDevfileSeparatedKubeCompsRevHistory,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &setRevHistoryLimit,
					Replicas:             &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only route",
			devfileString: kubernetesInlinedDevfileRoute,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"test":                         "test",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "route111222",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "component-sample",
						Weight: &weight,
					},
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only route, no targetport - should not panic",
			devfileString: kubernetesInlinedDevfileRouteNoTargetPort,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"test":                         "test",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "route111222",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "component-sample",
						Weight: &weight,
					},
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only Ingress",
			devfileString: kubernetesInlinedDevfileIngress,
			componentName: "component-sample",
			appName:       "application-sample",
			hostname:      host,
			image:         "image1",
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"test":                         "test",
					},
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/rewrite-target": "/",
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
											Path:     "/testpath",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test",
													Port: networkingv1.ServiceBackendPort{
														Number: 5566,
													},
												},
											},
										},
									},
								},
							},
							Host: "foo.bar.com",
						},
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only Svc",
			devfileString: kubernetesInlinedDevfileSvc,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with Deploy",
			devfileString: kubernetesInlinedDevfileDeploy,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Devfile with long component name - route name should be trimmed",
			devfileString: kubernetesInlinedDevfileDeploy,
			componentName: "component-sample-component-sample-component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample-component-sample-component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample-component-sample-component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample-component-sample-component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample-component-sample-component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample-component-sample-component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample-component-sample-component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample-component-sample-component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample-component-sample-component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
													Name: "component-sample-component-sample-component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample-component-sample-component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample-component-sample-component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample-component-sample-component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample-component-sample-component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with Route Host missing",
			devfileString: kubernetesInlinedDevfileRouteHostMissing,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
										{
											ContainerPort: int32(5566),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("400Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("200Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 5566,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile without inline",
			devfileString: kubernetesWithoutInline,
			componentName: "component-sample",
			image:         "image1",
		},
		{
			name:          "Simple devfile from Inline with multiple kubernetes components and only one is referenced by deploy command",
			devfileString: multipleKubernetesCompsDevfile,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only one kubernetes component but no deploy command",
			devfileString: kubernetesCompsWithNoDeployCmdDevfile,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
		},
		{
			name:          "No kubernetes components defined.",
			devfileString: noKubernetesCompDevfile,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Memory Limit",
			devfileString: kubernetesInlinedDevfileErrCase_BadMemoryLimit,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Storage Limit",
			devfileString: kubernetesInlinedDevfileErrCase_BadStorageLimit,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad CPU Limit",
			devfileString: kubernetesInlinedDevfileErrCase_BadCPULimit,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Memory Request",
			devfileString: kubernetesInlinedDevfileErrCase_BadMemoryRequest,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Storage Request",
			devfileString: kubernetesInlinedDevfileErrCase_BadStorageRequest,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad CPU Request",
			devfileString: kubernetesInlinedDevfileErrCase_BadCPUrequest,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var devfileSrc cdqanalysis.DevfileSrc
			if tt.devfileString != "" {
				devfileSrc = cdqanalysis.DevfileSrc{
					Data: tt.devfileString,
				}
			}

			devfileData, err := cdqanalysis.ParseDevfile(devfileSrc)
			if err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected parse error: %v", err)
			}
			deployAssociatedComponents, err := parser.GetDeployComponents(devfileData)
			if err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected get deploy components error: %v", err)
			}
			logger := ctrl.Log.WithName("TestGetResourceFromDevfile")

			actualResources, err := GetResourceFromDevfile(logger, devfileData, deployAssociatedComponents, tt.componentName, tt.appName, tt.image, tt.hostname)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected get resource from devfile error: %v", err)
			} else if err == nil {
				if len(actualResources.Deployments) > 0 {
					assert.Equal(t, tt.wantDeploy, actualResources.Deployments[0], "First Deployment did not match")
				}

				if len(actualResources.Services) > 0 {
					assert.Equal(t, tt.wantService, actualResources.Services[0], "First Service did not match")
				}

				if len(actualResources.Ingresses) > 0 {
					assert.Equal(t, tt.wantIngress, actualResources.Ingresses[0], "First Ingress did not match")
				}

				if len(actualResources.Routes) > 0 {
					if tt.name == "Devfile with long component name - route name should be trimmed" {
						if len(actualResources.Routes[0].Name) > 30 {
							t.Errorf("Expected trimmed route name with length < 30, but got %v", len(actualResources.Routes[0].Name))
						}
						if !strings.Contains(actualResources.Routes[0].Name, "component-sample-comp") {
							t.Errorf("Expected route name to contain %v, but got %v", "component-sample-comp", actualResources.Routes[0].Name)
						}
					} else {
						assert.Equal(t, tt.wantRoute, actualResources.Routes[0], "First Route did not match")
					}
				}
			}
		})
	}
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

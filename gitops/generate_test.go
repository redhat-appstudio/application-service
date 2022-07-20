//
// Copyright 2021-2022 Red Hat, Inc.
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

package gitops

import (
	"path/filepath"
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/resources"
	"github.com/redhat-appstudio/application-service/gitops/testutils"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/yaml"
)

func TestGenerateDeployment(t *testing.T) {
	applicationName := "test-application"
	componentName := "test-component"
	namespace := "test-namespace"
	replicas := int32(1)
	otherReplicas := int32(3)
	k8slabels := map[string]string{
		"app.kubernetes.io/name":       componentName,
		"app.kubernetes.io/instance":   componentName,
		"app.kubernetes.io/part-of":    applicationName,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
	matchLabels := map[string]string{
		"app.kubernetes.io/instance": componentName,
	}

	tests := []struct {
		name           string
		component      appstudiov1alpha1.Component
		wantDeployment appsv1.Deployment
	}{
		{
			name: "Simple component, no optional fields set",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
				},
			},
			wantDeployment: appsv1.Deployment{
				TypeMeta: v1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &v1.LabelSelector{
						MatchLabels: matchLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: matchLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "container-image",
									ImagePullPolicy: corev1.PullAlways,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Component, optional fields set",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  componentName,
					Application:    applicationName,
					Replicas:       3,
					TargetPort:     5000,
					ContainerImage: "quay.io/test/test-image:latest",
					Env: []corev1.EnvVar{
						{
							Name:  "test",
							Value: "value",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2M"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1M"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			wantDeployment: appsv1.Deployment{
				TypeMeta: v1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &otherReplicas,
					Selector: &v1.LabelSelector{
						MatchLabels: matchLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: matchLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "container-image",
									Image:           "quay.io/test/test-image:latest",
									ImagePullPolicy: corev1.PullAlways,
									Env: []corev1.EnvVar{
										{
											Name:  "test",
											Value: "value",
										},
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(5000),
										},
									},
									ReadinessProbe: &corev1.Probe{
										InitialDelaySeconds: 10,
										PeriodSeconds:       10,
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(5000),
											},
										},
									},
									LivenessProbe: &corev1.Probe{
										InitialDelaySeconds: 10,
										PeriodSeconds:       10,
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Port: intstr.FromInt(5000),
												Path: "/",
											},
										},
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("2M"),
											corev1.ResourceMemory: resource.MustParse("1Gi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("1M"),
											corev1.ResourceMemory: resource.MustParse("256Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Simple image component, no optional fields set",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  componentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/test:latest",
				},
			},
			wantDeployment: appsv1.Deployment{
				TypeMeta: v1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &v1.LabelSelector{
						MatchLabels: matchLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: matchLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "container-image",
									Image:           "quay.io/test/test:latest",
									ImagePullPolicy: corev1.PullAlways,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Simple image component with pull secret set",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  componentName,
					Application:    applicationName,
					Secret:         "my-image-pull-secret",
					ContainerImage: "quay.io/test/test:latest",
				},
			},
			wantDeployment: appsv1.Deployment{
				TypeMeta: v1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &v1.LabelSelector{
						MatchLabels: matchLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: matchLabels,
						},
						Spec: corev1.PodSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{
									Name: "my-image-pull-secret",
								},
							},
							Containers: []corev1.Container{
								{
									Name:            "container-image",
									Image:           "quay.io/test/test:latest",
									ImagePullPolicy: corev1.PullAlways,
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
			generatedDeployment := generateDeployment(tt.component)

			if !reflect.DeepEqual(*generatedDeployment, tt.wantDeployment) {
				t.Errorf("TestGenerateDeployment() error: expected %v got %v", tt.wantDeployment, generatedDeployment)
			}
		})
	}
}

func TestGenerateDeploymentPatch(t *testing.T) {
	componentName := "test-component"
	namespace := "test-namespace"
	replicas := int32(1)
	image := "image"

	tests := []struct {
		name           string
		component      appstudioshared.BindingComponent
		environment    appstudioshared.Environment
		imageName      string
		namespace      string
		wantDeployment appsv1.Deployment
	}{
		{
			name: "Simple component, no optional fields set",
			component: appstudioshared.BindingComponent{
				Name: componentName,
				Configuration: appstudioshared.BindingComponentConfiguration{
					Replicas: int(replicas),
					Env: []appstudioshared.EnvVarPair{
						{
							Name:  "FOO",
							Value: "BAR",
						},
					},
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			environment: appstudioshared.Environment{
				Spec: appstudioshared.EnvironmentSpec{
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR_ENV",
							},
							{
								Name:  "FOO2",
								Value: "BAR2_ENV",
							},
						},
					},
				},
			},
			namespace: namespace,
			imageName: image,
			wantDeployment: appsv1.Deployment{
				TypeMeta: v1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "container-image",
									Image: image,
									Env: []corev1.EnvVar{
										{
											Name:  "FOO",
											Value: "BAR",
										},
										{
											Name:  "FOO2",
											Value: "BAR2_ENV",
										},
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("1"),
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
			generatedDeployment := generateDeploymentPatch(tt.component, tt.environment, tt.imageName, tt.namespace)

			if !reflect.DeepEqual(*generatedDeployment, tt.wantDeployment) {
				t.Errorf("TestGenerateDeploymentPatch() error: expected %v got %v", tt.wantDeployment, *generatedDeployment)
			}
		})
	}
}

func TestGenerateService(t *testing.T) {
	applicationName := "test-application"
	componentName := "test-component"
	namespace := "test-namespace"
	k8slabels := map[string]string{
		"app.kubernetes.io/name":       componentName,
		"app.kubernetes.io/instance":   componentName,
		"app.kubernetes.io/part-of":    applicationName,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
	matchLabels := map[string]string{
		"app.kubernetes.io/instance": componentName,
	}

	tests := []struct {
		name        string
		component   appstudiov1alpha1.Component
		wantService corev1.Service
	}{
		{
			name: "Simple component object",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					TargetPort:    5000,
				},
			},
			wantService: corev1.Service{
				TypeMeta: v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: corev1.ServiceSpec{
					Selector: matchLabels,
					Ports: []corev1.ServicePort{
						{
							Port:       int32(5000),
							TargetPort: intstr.FromInt(5000),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generatedService := generateService(tt.component)

			if !reflect.DeepEqual(*generatedService, tt.wantService) {
				t.Errorf("TestGenerateService() error: expected %v got %v", tt.wantService, generatedService)
			}
		})
	}
}

func TestGenerateRoute(t *testing.T) {
	applicationName := "test-application"
	componentName := "test-component"
	namespace := "test-namespace"
	k8slabels := map[string]string{
		"app.kubernetes.io/name":       componentName,
		"app.kubernetes.io/instance":   componentName,
		"app.kubernetes.io/part-of":    applicationName,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
	weight := int32(100)

	tests := []struct {
		name      string
		component appstudiov1alpha1.Component
		wantRoute routev1.Route
	}{
		{
			name: "Simple component object",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					TargetPort:    5000,
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: v1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: routev1.RouteSpec{
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5000),
					},
					TLS: &routev1.TLSConfig{
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
						Termination:                   routev1.TLSTerminationEdge,
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   componentName,
						Weight: &weight,
					},
				},
			},
		},
		{
			name: "Component object with route/hostname set",
			component: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					TargetPort:    5000,
					Route:         "example.com",
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: v1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      componentName,
					Namespace: namespace,
					Labels:    k8slabels,
				},
				Spec: routev1.RouteSpec{
					Host: "example.com",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5000),
					},
					TLS: &routev1.TLSConfig{
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
						Termination:                   routev1.TLSTerminationEdge,
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   componentName,
						Weight: &weight,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generatedRoute := generateRoute(tt.component)

			if !reflect.DeepEqual(*generatedRoute, tt.wantRoute) {
				t.Errorf("TestGenerateRoute() error: expected %v got %v", tt.wantRoute, generatedRoute)
			}
		})
	}
}

func TestGenerateParentKustomize(t *testing.T) {
	fs := ioutils.NewMemoryFilesystem()

	// Prepopulate the fs with components
	gitOpsFolder := "/tmp/gitops"
	componentsFolder := filepath.Join(gitOpsFolder, "components")
	fs.MkdirAll(componentsFolder, 0755)
	fs.Mkdir(filepath.Join(componentsFolder, "compA"), 0755)
	fs.Mkdir(filepath.Join(componentsFolder, "compB"), 0755)
	fs.Mkdir(filepath.Join(componentsFolder, "compC"), 0755)

	tests := []struct {
		name    string
		fs      afero.Afero
		wantErr bool
	}{
		{
			name:    "Simple gitops repo with 3 components",
			fs:      fs,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateParentKustomize(tt.fs, gitOpsFolder)

			if tt.wantErr != (err != nil) {
				t.Errorf("unexpected error return value. Got %v", err)
			}

			if !tt.wantErr {
				// Validate that the kustomization.yaml got created successfully and contains the proper entries
				kustomizationFilepath := filepath.Join(gitOpsFolder, "kustomization.yaml")
				exists, err := tt.fs.Exists(kustomizationFilepath)
				if err != nil {
					t.Errorf("unexpected error checking if parent kustomize file exists %v", err)
				}
				if !exists {
					t.Errorf("parent kustomize file does not exist at path %v", kustomizationFilepath)
				}

				// Read the kustomization.yaml and validate its entries
				k := resources.Kustomization{}
				kustomizationBytes, err := tt.fs.ReadFile(kustomizationFilepath)
				if err != nil {
					t.Errorf("unexpected error reading parent kustomize file")
				}
				yaml.Unmarshal(kustomizationBytes, &k)

				// There should be 3 entries in the kustomization file
				if len(k.Bases) != 3 {
					t.Errorf("expected %v kustomization bases, got %v", 3, len(k.Bases))
				}

				// Validate that the APIVersion and Kind are set properly
				if k.Kind != "Kustomization" {
					t.Errorf("expected kustomize kind %v, got %v", "Kustomization", k.Kind)
				}
				if k.APIVersion != "kustomize.config.k8s.io/v1beta1" {
					t.Errorf("expected kustomize apiversion %v, got %v", "kustomize.config.k8s.io/v1beta1", k.APIVersion)
				}

			}
		})
	}
}

func TestGenerateOverlays(t *testing.T) {
	fs := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()

	// Prepopulate the fs with components
	gitOpsFolder := "/tmp/gitops"
	outputFolder := filepath.Join(gitOpsFolder, "overlays")
	fs.MkdirAll(outputFolder, 0755)

	outputFolderWithKustomizationFile := filepath.Join(gitOpsFolder, "overlays-2")
	fs.MkdirAll(outputFolderWithKustomizationFile, 0755)
	preExistKustomizationFilepath := filepath.Join(outputFolderWithKustomizationFile, "kustomization.yaml")
	k := resources.Kustomization{
		Patches: []string{"patch1.yaml", "custom-patch1.yaml"},
	}
	bytes, err := yaml.Marshal(k)
	if err != nil {
		t.Errorf("unexpected error when marshal the kustomization yaml %v", err)
	}
	err = fs.WriteFile(preExistKustomizationFilepath, bytes, 0755)
	if err != nil {
		t.Errorf("unexpected error when writing to kustomizatipn file: %v", err)
	}

	invalidKustomizationFileFolder := filepath.Join(gitOpsFolder, "overlays-error")
	fs.MkdirAll(invalidKustomizationFileFolder, 0755)
	invalidKustomizationFilepath := filepath.Join(invalidKustomizationFileFolder, "kustomization.yaml")
	invalidKustomization := map[string]interface{}{
		"Resources": 8,
	}
	bytes, err = yaml.Marshal(invalidKustomization)
	if err != nil {
		t.Errorf("unexpected error when marshal the kustomization yaml %v", err)
	}
	err = fs.WriteFile(invalidKustomizationFilepath, bytes, 0755)
	if err != nil {
		t.Errorf("unexpected error when writing to kustomizatipn file: %v", err)
	}

	component := appstudioshared.BindingComponent{
		Name:          "test-component",
		Configuration: appstudioshared.BindingComponentConfiguration{},
	}
	imageName := "test-image"
	namespace := "test-namespace"
	environment := appstudioshared.Environment{}

	tests := []struct {
		name                        string
		fs                          afero.Afero
		outputFolder                string
		expectPatchEntries          int
		componentGeneratedResources map[string][]string
		wantErr                     string
	}{
		{
			name:               "simple success case",
			fs:                 fs,
			outputFolder:       outputFolder,
			expectPatchEntries: 1,
			wantErr:            "",
		},
		{
			name:               "existing kustomization file with custom patches",
			fs:                 fs,
			outputFolder:       outputFolderWithKustomizationFile,
			expectPatchEntries: 3,
			wantErr:            "",
		},
		{
			name:         "read only fs",
			fs:           readOnlyFs,
			outputFolder: outputFolderWithKustomizationFile,
			wantErr:      "failed to MkDirAll",
		},
		{
			name:         "unmarshall error",
			fs:           fs,
			outputFolder: invalidKustomizationFileFolder,
			wantErr:      " failed to unmarshal data: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal number into Go struct field Kustomization.resources",
		},
		{
			name:         "genereated an additional patch",
			fs:           fs,
			outputFolder: outputFolderWithKustomizationFile,
			componentGeneratedResources: map[string][]string{
				"test-component": {
					"patch1.yaml",
				},
			},
			expectPatchEntries: 2,
			wantErr:            "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateOverlays(tt.fs, gitOpsFolder, tt.outputFolder, component, environment, imageName, namespace, tt.componentGeneratedResources)

			if !testutils.ErrorMatch(t, tt.wantErr, err) {
				t.Errorf("unexpected error return value. Got %v", err)
			}

			if tt.wantErr == "" {
				// Validate that the kustomization.yaml got created successfully and contains the proper entries
				kustomizationFilepath := filepath.Join(tt.outputFolder, "kustomization.yaml")
				exists, err := tt.fs.Exists(kustomizationFilepath)
				if err != nil {
					t.Errorf("unexpected error checking if kustomize file exists %v", err)
				}
				if !exists {
					t.Errorf("kustomize file does not exist at path %v", kustomizationFilepath)
				}

				// Read the kustomization.yaml and validate its entries
				k := resources.Kustomization{}
				kustomizationBytes, err := tt.fs.ReadFile(kustomizationFilepath)
				if err != nil {
					t.Errorf("unexpected error reading parent kustomize file")
				}
				yaml.Unmarshal(kustomizationBytes, &k)

				// There match patch entries in the kustomization file
				if len(k.Patches) != tt.expectPatchEntries {
					t.Errorf("expected %v kustomization bases, got %v", 3, len(k.Patches))
				}

				// Validate that the APIVersion and Kind are set properly
				if k.Kind != "Kustomization" {
					t.Errorf("expected kustomize kind %v, got %v", "Kustomization", k.Kind)
				}
				if k.APIVersion != "kustomize.config.k8s.io/v1beta1" {
					t.Errorf("expected kustomize apiversion %v, got %v", "kustomize.config.k8s.io/v1beta1", k.APIVersion)
				}

			}
		})
	}
}

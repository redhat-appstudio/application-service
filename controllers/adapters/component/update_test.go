package component

import (
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	testutil "github.com/redhat-appstudio/application-service/pkg/testutil"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	devfilePkg "github.com/redhat-appstudio/application-service/pkg/devfile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestUpdateComponentDevfileModel(t *testing.T) {

	storage1GiResource, err := resource.ParseQuantity("1Gi")
	if err != nil {
		t.Error(err)
	}
	core500mResource, err := resource.ParseQuantity("500m")
	if err != nil {
		t.Error(err)
	}

	originalResources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:     core500mResource,
			corev1.ResourceMemory:  storage1GiResource,
			corev1.ResourceStorage: storage1GiResource,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:     core500mResource,
			corev1.ResourceMemory:  storage1GiResource,
			corev1.ResourceStorage: storage1GiResource,
		},
	}

	envAttributes := attributes.Attributes{}.FromMap(map[string]interface{}{devfilePkg.ContainerENVKey: []corev1.EnvVar{{Name: "FOO", Value: "foo"}}}, &err)
	if err != nil {
		t.Error(err)
	}

	env := []corev1.EnvVar{
		{
			Name:  "FOO",
			Value: "foo1",
		},
		{
			Name:  "BAR",
			Value: "bar1",
		},
	}

	tests := []struct {
		name           string
		components     []devfileAPIV1.Component
		component      appstudiov1alpha1.Component
		updateExpected bool
		wantErr        bool
	}{
		{
			name: "No kubernetes component",
			components: []devfileAPIV1.Component{
				{
					Name: "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Container: &devfileAPIV1.ContainerComponent{},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
				},
			},
		},
		{
			name: "one kubernetes component",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 1001),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
					Application:   "applicationName",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
					Route:      "route1",
					Replicas:   1,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
				},
			},
			updateExpected: true,
		},
		{
			name: "two kubernetes components",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 1001),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
				{
					Name:       "component2",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 3333).PutString(devfilePkg.MemoryLimitKey, "2Gi"),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
					Application:   "applicationName",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
					Route:      "route1",
					Replicas:   1,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
				},
			},
			updateExpected: true,
		},
		{
			name: "Component with envFrom component - should error out as it's not supported right now",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes,
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Env: []corev1.EnvVar{
						{
							Name:  "FOO",
							Value: "foo",
						},
						{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: "test",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Component with invalid component type - should error out",
			components: []devfileAPIV1.Component{
				{
					Name:           "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Env: []corev1.EnvVar{
						{
							Name:  "FOO",
							Value: "foo",
						},
						{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: "test",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "image component with local dockerfile uri updated to component's absolute dockerfileURL",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 1001),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
				{
					Name:       "component2",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 3333).PutString(devfilePkg.MemoryLimitKey, "2Gi"),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Image: &devfileAPIV1.ImageComponent{

							Image: devfileAPIV1.Image{
								ImageUnion: devfileAPIV1.ImageUnion{
									Dockerfile: &devfileAPIV1.DockerfileImage{
										DockerfileSrc: devfileAPIV1.DockerfileSrc{
											Uri: "./dockerfile",
										},
									},
								},
							},
						},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
					Application:   "applicationName",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:           "url",
								DockerfileURL: "https://website.com/dockerfiles/dockerfile",
							},
						},
					},
					Route:      "route1",
					Replicas:   1,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
				},
			},
			updateExpected: true,
		},
		{
			name: "devfile with invalid components, error out when trying to update devfile's dockerfile uri",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 1001),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						ComponentType: "bad-component",
					},
				},
				{
					Name:       "component2",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 3333).PutString(devfilePkg.MemoryLimitKey, "2Gi"),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Image: &devfileAPIV1.ImageComponent{

							Image: devfileAPIV1.Image{
								ImageUnion: devfileAPIV1.ImageUnion{
									Dockerfile: &devfileAPIV1.DockerfileImage{
										DockerfileSrc: devfileAPIV1.DockerfileSrc{
											Uri: "./dockerfile",
										},
									},
								},
							},
						},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
					Application:   "applicationName",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:           "url",
								DockerfileURL: "https://website.com/dockerfiles/dockerfile",
							},
						},
					},
					Route:      "route1",
					Replicas:   1,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileData := &v2.DevfileV2{
				Devfile: devfileAPIV1.Devfile{
					DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
							Components: tt.components,
						},
					},
				},
			}

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{
				Development: true,
			})))
			a := Adapter{
				Log: ctrl.Log.WithName("TestUpdateComponentDevfileModel"),
			}
			err := a.updateComponentDevfileModel(devfileData, tt.component)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				if tt.updateExpected {
					// it has been updated
					checklist := testutil.UpdateChecklist{
						Route:     tt.component.Spec.Route,
						Replica:   tt.component.Spec.Replicas,
						Port:      tt.component.Spec.TargetPort,
						Env:       tt.component.Spec.Env,
						Resources: tt.component.Spec.Resources,
					}

					testutil.VerifyHASComponentUpdates(devfileData, checklist, t)
				}
			}
		})
	}
}

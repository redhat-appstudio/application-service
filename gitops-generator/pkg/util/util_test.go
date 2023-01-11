package util

import (
	"reflect"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetMappedComponent(t *testing.T) {

	tests := []struct {
		name      string
		component appstudiov1alpha1.Component
		want      gitopsgenv1alpha1.GeneratorOptions
	}{
		{
			name: "Test01ComponentSpecFilledIn",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest001",
					Secret:        "Secret",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceLimitsCPU: resource.MustParse("1"),
							corev1.ResourceMemory:    resource.MustParse("1Gi"),
						},
					},
					TargetPort: 8080,
					Route:      "https://testroute",
					Env: []corev1.EnvVar{
						{
							Name:  "env1",
							Value: "env1Value",
						},
					},
					ContainerImage:               "myimage:image",
					SkipGitOpsResourceGeneration: false,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:           "https://host/git-repo.git",
								Revision:      "1.0",
								Context:       "/context",
								DevfileURL:    "https://mydevfileurl",
								DockerfileURL: "https://mydockerfileurl",
							},
						},
					},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Namespace:   "testnamespace",
				Application: "AppTest001",
				Secret:      "Secret",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceLimitsCPU: resource.MustParse("1"),
						corev1.ResourceMemory:    resource.MustParse("1Gi"),
					},
				},
				TargetPort: 8080,
				Route:      "https://testroute",
				BaseEnvVar: []corev1.EnvVar{
					{
						Name:  "env1",
						Value: "env1Value",
					},
				},
				ContainerImage: "myimage:image",
				GitSource: &gitopsgenv1alpha1.GitSource{
					URL: "https://host/git-repo.git",
				},
			},
		},
		{
			name: "Test02EmptyComponentSource",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest002",
					Source:        appstudiov1alpha1.ComponentSource{},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Namespace:   "testnamespace",
				Application: "AppTest002",
				GitSource:   &gitopsgenv1alpha1.GitSource{},
			},
		},
		{
			name: "Test03NoSource",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest003",
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Namespace:   "testnamespace",
				Application: "AppTest003",
			},
		},
		{
			name: "Test04EmptyComponentSourceUnion",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest004",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{},
					},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Namespace:   "testnamespace",
				Application: "AppTest004",
				GitSource:   &gitopsgenv1alpha1.GitSource{},
			},
		},
		{
			name: "Test05EmptyGitSource",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest005",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
						},
					},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Namespace:   "testnamespace",
				Application: "AppTest005",
				GitSource:   &gitopsgenv1alpha1.GitSource{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedComponent := GetMappedGitOpsComponent(tt.component)
			assert.True(t, tt.want.Name == mappedComponent.Name, "Expected ObjectMeta.Name: %s, is different than actual: %s", tt.want.Name, mappedComponent.Name)
			assert.True(t, tt.want.Namespace == mappedComponent.Namespace, "Expected ObjectMeta.Namespace: %s, is different than actual: %s", tt.want.Namespace, mappedComponent.Namespace)
			assert.True(t, tt.want.Application == mappedComponent.Application, "Expected Spec.Application: %s, is different than actual: %s", tt.want.Application, mappedComponent.Application)
			assert.True(t, tt.want.Secret == mappedComponent.Secret, "Expected Spec.Secret: %s, is different than actual: %s", tt.want.Secret, mappedComponent.Secret)
			assert.True(t, reflect.DeepEqual(tt.want.Resources, mappedComponent.Resources), "Expected Spec.Resources: %s, is different than actual: %s", tt.want.Resources, mappedComponent.Resources)
			assert.True(t, tt.want.Route == mappedComponent.Route, "Expected Spec.Route: %s, is different than actual: %s", tt.want.Route, mappedComponent.Route)
			assert.True(t, reflect.DeepEqual(tt.want.BaseEnvVar, mappedComponent.BaseEnvVar), "Expected Spec.Env: %s, is different than actual: %s", tt.want.BaseEnvVar, mappedComponent.BaseEnvVar)
			assert.True(t, tt.want.ContainerImage == mappedComponent.ContainerImage, "Expected Spec.ContainerImage: %s, is different than actual: %s", tt.want.ContainerImage, mappedComponent.ContainerImage)

			if tt.want.GitSource != nil {
				assert.True(t, tt.want.GitSource.URL == mappedComponent.GitSource.URL, "Expected GitSource URL: %s, is different than actual: %s", tt.want.GitSource.URL, mappedComponent.GitSource.URL)
			}
		})
	}
}

func TestGetRemoteURL(t *testing.T) {
	tests := []struct {
		name      string
		gitopsUrl string
		token     string
		want      string
		wantErr   bool
	}{
		{
			name:      "Basic URL",
			gitopsUrl: "https://github.com/redhat-appstudio-appdata/test",
			token:     "my-token",
			want:      "https://my-token@github.com/redhat-appstudio-appdata/test",
		},
		{
			name:      "Invalid URL",
			gitopsUrl: "http://github.com/?org\nrepo",
			token:     "my-token",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote, err := GetRemoteURL(tt.gitopsUrl, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGetRemoteURL: unexpected error %v", err)
			}
			if !tt.wantErr && (remote != tt.want) {
				t.Errorf("TestGetRemoteURL: want %v, got %v", tt.want, remote)
			}
		})
	}
}

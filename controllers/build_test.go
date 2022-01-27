/*
Copyright 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controllers

import (
	"reflect"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_normalizeOutputImageURL(t *testing.T) {
	type args struct {
		outputImage string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "not a fully qualified url",
			args: args{
				outputImage: "quay.io/foo/bar",
			},
			want: "quay.io/foo/bar:$(tt.params.git-revision)",
		},
		{
			name: "fully qualified url",
			args: args{
				outputImage: "quay.io/foo/bar:latest",
			},
			want: "quay.io/foo/bar:latest-$(tt.params.git-revision)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOutputImageURL(tt.args.outputImage); got != tt.want {
				t.Errorf("normalizeOutputImageURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_determineBuildExecution(t *testing.T) {
	type args struct {
		component        appstudiov1alpha1.Component
		params           []tektonapi.Param
		workspaceSubPath string
	}
	tests := []struct {
		name string
		args args
		want tektonapi.PipelineRunSpec
	}{
		{
			name: "for non webhooks",
			args: args{
				component: appstudiov1alpha1.Component{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testcomponent",
						Namespace: "kcpworkspacename",
					},
				},
				workspaceSubPath: "initialbuild",
				params:           []tektonapi.Param{},
			},
			want: tektonapi.PipelineRunSpec{
				PipelineRef: &tektonapi.PipelineRef{
					Bundle: "quay.io/redhat-appstudio/build-templates-bundle:v0.1.2",
					Name:   "devfile-build",
				},
				Params: []tektonapi.Param{},
				Workspaces: []tektonapi.WorkspaceBinding{
					{
						Name: "workspace",
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "appstudio",
						},
						SubPath: "testcomponent/initialbuild",
					},
					{
						Name: "registry-auth",
						Secret: &corev1.SecretVolumeSource{
							SecretName: "redhat-appstudio-registry-pull-secret",
						},
					},
				},
			},
		},
		{
			name: "for webhooks",
			args: args{
				component: appstudiov1alpha1.Component{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testcomponent",
						Namespace: "kcpworkspacename",
					},
				},
				workspaceSubPath: "a-long-git-reference",
				params:           []tektonapi.Param{},
			},
			want: tektonapi.PipelineRunSpec{
				PipelineRef: &tektonapi.PipelineRef{
					Bundle: "quay.io/redhat-appstudio/build-templates-bundle:v0.1.2",
					Name:   "devfile-build",
				},
				Params: []tektonapi.Param{},
				Workspaces: []tektonapi.WorkspaceBinding{
					{
						Name: "workspace",
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "appstudio",
						},
						SubPath: "testcomponent/a-long-git-reference",
					},
					{
						Name: "registry-auth",
						Secret: &corev1.SecretVolumeSource{
							SecretName: "redhat-appstudio-registry-pull-secret",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineBuildExecution(tt.args.component, tt.args.params, tt.args.workspaceSubPath); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("determineBuildExecution() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_paramsForInitialBuild(t *testing.T) {
	type args struct {
		component appstudiov1alpha1.Component
	}
	tests := []struct {
		name string
		args args
		want []tektonapi.Param
	}{
		{
			name: "use the image as is",
			args: args{
				component: appstudiov1alpha1.Component{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testcomponent",
						Namespace: "kcpworkspacename",
					},
					Spec: appstudiov1alpha1.ComponentSpec{
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									URL: "https://a/b/c",
								},
							},
						},
						Build: appstudiov1alpha1.Build{
							ContainerImage: "whatever-is-set",
						},
					},
				},
			},
			want: []tektonapi.Param{
				{
					Name: "git-url",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "https://a/b/c",
					},
				},
				{
					Name: "output-image",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "whatever-is-set",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paramsForInitialBuild(tt.args.component); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("paramsForInitialBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_paramsForWebhookBasedBuilds(t *testing.T) {
	type args struct {
		component appstudiov1alpha1.Component
	}
	tests := []struct {
		name string
		args args
		want []tektonapi.Param
	}{
		{
			name: "use the updated image tag",
			args: args{
				component: appstudiov1alpha1.Component{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testcomponent",
						Namespace: "kcpworkspacename",
					},
					Spec: appstudiov1alpha1.ComponentSpec{
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									URL: "https://a/b/c",
								},
							},
						},
						Build: appstudiov1alpha1.Build{
							ContainerImage: "docker.io/foo/bar:tag",
						},
					},
				},
			},
			want: []tektonapi.Param{
				{
					Name: "git-url",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "https://a/b/c",
					},
				},
				{
					Name: "output-image",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "docker.io/foo/bar:tag-$(tt.params.git-revision)",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paramsForWebhookBasedBuilds(tt.args.component); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("paramsForWebhookBasedBuilds() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
package gitops

import (
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/prepare"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func devfileToString(devfile data.DevfileData) string {
	yamlDevfile, err := yaml.Marshal(devfile)
	if err != nil {
		panic("Invalid test devfile")
	}
	return string(yamlDevfile)
}

func getSampleDevfileComponents() []v1alpha2.Component {
	return []v1alpha2.Component{
		{
			Name: "outerloop-deploy",
			ComponentUnion: v1alpha2.ComponentUnion{
				Kubernetes: &v1alpha2.KubernetesComponent{
					K8sLikeComponent: v1alpha2.K8sLikeComponent{
						K8sLikeComponentLocation: v1alpha2.K8sLikeComponentLocation{
							Uri: "test-uri",
						},
					},
				},
			},
		},
		{
			Name: "outerloop-build",
			ComponentUnion: v1alpha2.ComponentUnion{
				Image: &v1alpha2.ImageComponent{
					Image: v1alpha2.Image{
						ImageUnion: v1alpha2.ImageUnion{
							Dockerfile: &v1alpha2.DockerfileImage{
								DockerfileSrc: v1alpha2.DockerfileSrc{
									Uri: "dockerfile-uri",
								},
								Dockerfile: v1alpha2.Dockerfile{
									BuildContext: "build-context-path",
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestNormalizeOutputImageURL(t *testing.T) {
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

func TestGenerateInitialBuildPipelineRun(t *testing.T) {
	component := appstudiov1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testcomponent",
			Namespace: "kcpworkspacename",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL: "https://host/git-repo",
					},
				},
			},
		},
	}

	gitopsConfig := prepare.GitopsConfig{BuildBundle: "quay.io/redhat-appstudio/build-templates-bundle:0.0.1"}

	type args struct {
		component appstudiov1alpha1.Component
	}
	tests := []struct {
		name string
		args args
		want tektonapi.PipelineRun
	}{
		{
			name: "generate initial build pipelien run",
			args: args{
				component: component,
			},
			want: tektonapi.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "testcomponent-",
					Namespace:    "kcpworkspacename",
					Labels:       getBuildCommonLabelsForComponent(&component),
				},
				Spec: tektonapi.PipelineRunSpec{
					PipelineRef: &tektonapi.PipelineRef{
						Bundle: gitopsConfig.BuildBundle,
						Name:   "noop",
					},
					Params: []tektonapi.Param{
						{
							Name: "git-url",
							Value: tektonapi.ArrayOrString{
								Type:      tektonapi.ParamTypeString,
								StringVal: "https://host/git-repo",
							},
						},
						{
							Name: "output-image",
							Value: tektonapi.ArrayOrString{
								Type:      tektonapi.ParamTypeString,
								StringVal: "",
							},
						},
					},
					Workspaces: []tektonapi.WorkspaceBinding{
						{
							Name: "workspace",
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "appstudio",
							},
							SubPath: "testcomponent/" + getInitialBuildWorkspaceSubpath(),
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateInitialBuildPipelineRun(tt.args.component, gitopsConfig); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateInitialBuildPipelineRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineBuildExecution(t *testing.T) {
	type args struct {
		component        appstudiov1alpha1.Component
		params           []tektonapi.Param
		workspaceSubPath string
	}

	gitopsConfig := prepare.GitopsConfig{BuildBundle: "quay.io/redhat-appstudio/build-templates-bundle:0.0.1"}

	tests := []struct {
		name string
		args args
		want tektonapi.PipelineRunSpec
	}{
		{
			name: "for non webhooks",
			args: args{
				component: appstudiov1alpha1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testcomponent",
						Namespace: "kcpworkspacename",
					},
				},
				workspaceSubPath: "initialbuild",
				params:           []tektonapi.Param{},
			},
			want: tektonapi.PipelineRunSpec{
				PipelineRef: &tektonapi.PipelineRef{
					Bundle: gitopsConfig.BuildBundle,
					Name:   "noop",
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
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testcomponent",
						Namespace: "kcpworkspacename",
					},
				},
				workspaceSubPath: "a-long-git-reference",
				params:           []tektonapi.Param{},
			},
			want: tektonapi.PipelineRunSpec{
				PipelineRef: &tektonapi.PipelineRef{
					Bundle: gitopsConfig.BuildBundle,
					Name:   "noop",
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
			if got := DetermineBuildExecution(tt.args.component, tt.args.params, tt.args.workspaceSubPath, gitopsConfig); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DetermineBuildExecution() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineBuildPipeline(t *testing.T) {
	createDevfileWithBuildInfo := func(language string, projectType string) data.DevfileData {
		devfileVersion := string(data.APISchemaVersion220)
		devfileData, _ := data.NewDevfileData(devfileVersion)
		devfileData.SetSchemaVersion(devfileVersion)
		devfileData.SetMetadata(devfile.DevfileMetadata{
			Name:        "test-devfile",
			Language:    language,
			ProjectType: projectType,
		})
		return devfileData
	}
	createDevfileStatusModelWithBuildInfo := func(language string, projectType string) string {
		return devfileToString(createDevfileWithBuildInfo(language, projectType))
	}
	createDevfileWithoutBuildInfoButWithDockerfileComponent := func() string {
		devfileData := createDevfileWithBuildInfo("java", "")
		devfileData.AddComponents(getSampleDevfileComponents())
		return devfileToString(devfileData)
	}

	tests := []struct {
		name      string
		component appstudiov1alpha1.Component
		want      string
	}{
		{
			name: "should use java builder",
			component: appstudiov1alpha1.Component{
				Status: appstudiov1alpha1.ComponentStatus{
					Devfile: createDevfileStatusModelWithBuildInfo("java", "quarkus"),
				},
			},
			want: "java-builder",
		},
		{
			name: "should use nodejs builder",
			component: appstudiov1alpha1.Component{
				Status: appstudiov1alpha1.ComponentStatus{
					Devfile: createDevfileStatusModelWithBuildInfo("nodejs", ""),
				},
			},
			want: "nodejs-builder",
		},
		{
			name: "should use python builder",
			component: appstudiov1alpha1.Component{
				Status: appstudiov1alpha1.ComponentStatus{
					Devfile: createDevfileStatusModelWithBuildInfo("python", "django"),
				},
			},
			// TODO fix when python builder is in place
			want: "noop",
		},
		{
			name: "should use noop builder if failed to determine pipeline",
			component: appstudiov1alpha1.Component{
				Status: appstudiov1alpha1.ComponentStatus{
					Devfile: createDevfileStatusModelWithBuildInfo("unknown", ""),
				},
			},
			want: "noop",
		},
		{
			name: "should use docker builder if dockerfile present",
			component: appstudiov1alpha1.Component{
				Status: appstudiov1alpha1.ComponentStatus{
					Devfile: createDevfileWithoutBuildInfoButWithDockerfileComponent(),
				},
			},
			want: "docker-build",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineBuildPipeline(tt.component); got != tt.want {
				t.Errorf("determineBuildPipeline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetParamsForComponentBuild(t *testing.T) {
	getDevfileWithOuterloopBuildDockerfile := func() string {
		devfileVersion := string(data.APISchemaVersion220)
		devfileData, _ := data.NewDevfileData(devfileVersion)
		devfileData.SetSchemaVersion(devfileVersion)
		devfileData.AddComponents(getSampleDevfileComponents())
		return devfileToString(devfileData)
	}

	tests := []struct {
		name           string
		IsInitialBuild bool
		component      appstudiov1alpha1.Component
		want           []tektonapi.Param
	}{
		{
			name:           "use the image as is",
			IsInitialBuild: true,
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "kcpworkspacename",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: "whatever-is-set",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://a/b/c",
							},
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

		{
			name:           "use the updated image tag",
			IsInitialBuild: false,
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "kcpworkspacename",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: "docker.io/foo/bar:tag",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://a/b/c",
							},
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

		{
			name:           "set dockerfile path and context",
			IsInitialBuild: false,
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "kcpworkspacename",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: "docker.io/foo/bar:tag",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://a/b/c",
							},
						},
					},
				},
				Status: appstudiov1alpha1.ComponentStatus{
					Devfile: getDevfileWithOuterloopBuildDockerfile(),
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
				{
					Name: "dockerfile",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "dockerfile-uri",
					},
				},
				{
					Name: "path-context",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "build-context-path",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getParamsForComponentBuild(tt.component, tt.IsInitialBuild); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetParamsForComponentBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

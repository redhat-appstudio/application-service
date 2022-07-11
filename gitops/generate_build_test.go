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
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/devfile"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/mitchellh/go-homedir"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/prepare"
	"github.com/redhat-appstudio/application-service/gitops/testutils"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
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

func TestGenerateBuild(t *testing.T) {
	outoutFolder := "output"
	gitopsConfig := prepare.GitopsConfig{}

	tests := []struct {
		name      string
		fs        afero.Afero
		component appstudiov1alpha1.Component
		want      []string
	}{
		{
			name: "Check trigger based resources",
			fs:   ioutils.NewMemoryFilesystem(),
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "workspace-name",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://host/git-repo.git",
							},
						},
					},
				},
			},
			want: []string{
				kustomizeFileName,
				buildTriggerTemplateFileName,
				buildEventListenerFileName,
				buildWebhookRouteFileName,
			},
		},
		{
			name: "Check pipeline as code resources",
			fs:   ioutils.NewMemoryFilesystem(),
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "workspace-name",
					Annotations: map[string]string{
						PaCAnnotation: "1",
					},
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://host/git-repo.git",
							},
						},
					},
				},
			},
			want: []string{
				kustomizeFileName,
				buildRepositoryFileName,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := GenerateBuild(tt.fs, outoutFolder, tt.component, gitopsConfig); err != nil {
				t.Errorf("Failed to generate builf gitops resources. Cause: %v", err)
			}

			// Ensure that needed resources generated
			path, err := homedir.Expand(outoutFolder)
			testutils.AssertNoError(t, err)

			for _, item := range tt.want {
				exist, err := tt.fs.Exists(filepath.Join(path, item))
				testutils.AssertNoError(t, err)
				assert.True(t, exist, "Expected file %s missing in gitops", item)
			}
		})
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
			want: "quay.io/foo/bar:latest-$(tt.params.git-revision)",
		},
		{
			name: "fully qualified url",
			args: args{
				outputImage: "quay.io/foo/bar:latest",
			},
			want: "quay.io/foo/bar:latest-$(tt.params.git-revision)",
		},
		{
			name: "contains git revision suffix in tag",
			args: args{
				outputImage: "quay.io/foo/bar:tag-29b0823364ba05bd5a9d3a89d4e6cad57d2d3723",
			},
			want: "quay.io/foo/bar:tag-$(tt.params.git-revision)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOutputImageURL(tt.args.outputImage)
			if got != tt.want {
				t.Errorf("normalizeOutputImageURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProtectDefaultImageRepo(t *testing.T) {
	type args struct {
		outputImage string
	}
	tests := []struct {
		name        string
		namespace   string
		args        args
		want        string
		expectError bool
	}{
		{
			name:      "fully qualified url to default repo, exact matching user",
			namespace: "mytag",
			args: args{
				outputImage: DefaultImageRepo + ":mytag",
			},
			expectError: true,
		},
		{
			name:      "fully qualified url to default repo, matching user with suffix",
			namespace: "mytag",
			args: args{
				outputImage: DefaultImageRepo + ":mytag-test",
			},
		},
		{
			name:      "fully qualified url to default repo, mismatched users",
			namespace: "yourtag",
			args: args{
				outputImage: DefaultImageRepo + ":mytag",
			},
			expectError: true,
		},
		{
			name:      "fully qualified url to default repo, mismatched users with suffix",
			namespace: "yourtag",
			args: args{
				outputImage: DefaultImageRepo + ":mytag-test",
			},
			expectError: true,
		},
		{
			name:      "fully qualified url to default repo, pushing without tag",
			namespace: "yourtag",
			args: args{
				outputImage: DefaultImageRepo,
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := protectDefaultImageRepo(tt.args.outputImage, tt.namespace)
			if err == nil && tt.expectError {
				t.Errorf("protectDefaultImageRepo() expected error but got none")
			}
			if err != nil && !tt.expectError {
				t.Errorf("protectDefaultImageRepo() got unexpected error: %s", err.Error())
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

	buildBundle := "quay.io/redhat-appstudio/build-templates-bundle:0.0.1"

	type args struct {
		component appstudiov1alpha1.Component
	}
	tests := []struct {
		name                  string
		args                  args
		registrySecretMissing bool
		want                  tektonapi.PipelineRun
		expectError           bool
	}{
		{
			name: "generate initial build pipeline run",
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
						Bundle: buildBundle,
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
		{
			name:                  "generate initial build pipeline run no registry secret",
			registrySecretMissing: true,
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
						Bundle: buildBundle,
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
					},
				},
			},
		},
		{
			name:        "generate initial build pipeline run, protected default repo",
			expectError: true,
			args: args{
				component: appstudiov1alpha1.Component{
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
						ContainerImage: DefaultImageRepo + ":mytag",
					},
				},
			},
			want: tektonapi.PipelineRun{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitopsConfig := prepare.GitopsConfig{BuildBundle: buildBundle, AppStudioRegistrySecretPresent: !tt.registrySecretMissing}
			got, err := GenerateInitialBuildPipelineRun(tt.args.component, gitopsConfig)
			if err == nil && tt.expectError {
				t.Errorf("GenerateInitialBuildPipelineRun() expected error but got none")
			}
			if err != nil && !tt.expectError {
				t.Errorf("GenerateInitialBuildPipelineRun() got unexpected error: %s", err.Error())
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
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

	buildBundle := "quay.io/redhat-appstudio/build-templates-bundle:0.0.1"

	tests := []struct {
		name                  string
		args                  args
		registrySecretMissing bool
		want                  tektonapi.PipelineRunSpec
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
					Bundle: buildBundle,
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
					Bundle: buildBundle,
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
		{
			name:                  "no registry secret",
			registrySecretMissing: true,
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
					Bundle: buildBundle,
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
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitopsConfig := prepare.GitopsConfig{BuildBundle: buildBundle, AppStudioRegistrySecretPresent: !tt.registrySecretMissing}
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

func TestGenerateTriggerTemplate(t *testing.T) {
	tests := []struct {
		name         string
		component    appstudiov1alpha1.Component
		gitopsConfig prepare.GitopsConfig
		// given the byte serialization around the pipelinerun, we just verify the component related changes vs. deepequal
		wantErr bool
	}{
		{
			name: "working",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
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
				},
			},
		},
		{
			name:    "default repo mismatched user error on non initial build",
			wantErr: true,
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "kcpworkspacename",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: DefaultImageRepo + ":mytag",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://a/b/c",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateTriggerTemplate(tt.component, tt.gitopsConfig)
			if err != nil && !tt.wantErr {
				t.Errorf("GenerateTriggerTemplate() unexpected error: %s", err.Error())
			}
			if err == nil && tt.wantErr {
				t.Errorf("GenerateTriggerTemplate() did not get expected error")
			}
			if !tt.wantErr {
				if got == nil {
					t.Errorf("GenerateTriggerTemplate() nil trigger template")
				} else {
					// we employ the else here so staticcheck does not complain, since it does not understand what t.Errorf does
					if got.Namespace != tt.component.Namespace {
						t.Errorf("GenerateTriggerTemplate() namespace mismatch: got %s want %s", got.Namespace, tt.component.Namespace)
					}
					if got.Name != tt.component.Name {
						t.Errorf("GenerateTriggerTemplate() name mismatch: got %s want %s", got.Name, tt.component.Name)
					}
					// reverse engineer the PipelineRun
					var pr tektonapi.PipelineRun
					for _, rt := range got.Spec.ResourceTemplates {
						err := json.Unmarshal(rt.Raw, &pr)
						if err != nil {
							t.Errorf("GenerateTriggerTemplate() error unmarshalling pipelinerun: %s", err.Error())
						}
						if !strings.HasPrefix(pr.GenerateName, tt.component.Name) {
							t.Errorf("GenerateTriggerTemplate() generate name mismatch, got %s want prefix %s", pr.GenerateName, tt.component.Namespace)
						}
						if pr.Namespace != tt.component.Namespace {
							t.Errorf("GenerateTriggerTemplate() namespace mismatch: got %s want %s", pr.Namespace, tt.component.Namespace)
						}
						compA, ok := pr.Annotations["build.appstudio.openshift.io/component"]
						if !ok || compA != tt.component.Name {
							t.Errorf("GenerateTriggerTemplate() component annotation incorrect: %v %s", ok, compA)
						}
						appA, ok := pr.Annotations["build.appstudio.openshift.io/application"]
						if !ok || appA != tt.component.Spec.Application {
							t.Errorf("GenerateTriggerTemplate() app annotation incorrect: %v %s", ok, appA)
						}
					}
				}
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
		wantErr        bool
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
			name:           "Use Image as is, ensure revision is set",
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
								URL:      "https://a/b/c",
								Revision: "master",
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
				{
					Name: "revision",
					Value: tektonapi.ArrayOrString{
						Type:      tektonapi.ParamTypeString,
						StringVal: "master",
					},
				},
			},
		},

		{
			name:    "default repo mismatched user error on non initial build",
			wantErr: true,
			want:    []tektonapi.Param{},
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "kcpworkspacename",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: DefaultImageRepo + ":mytag",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://a/b/c",
							},
						},
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
		{
			name:    "default image repo with tag, not matching namespace",
			wantErr: true,
			want:    []tektonapi.Param{},
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "kcpworkspacename",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: DefaultImageRepo + ":mytag-test",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://a/b/c",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getParamsForComponentBuild(tt.component, tt.IsInitialBuild)
			if err != nil && !tt.wantErr {
				t.Errorf("GetParamsForComponentBuild() unexpected error: %s", err.Error())
			}
			if err == nil && tt.wantErr {
				t.Errorf("GetParamsForComponentBuild() did not get expected error")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetParamsForComponentBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeneratePACRepository(t *testing.T) {
	repoUrl := "https://host/git-repo.git"
	component := appstudiov1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testcomponent",
			Namespace: "workspace-name",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL: repoUrl,
					},
				},
			},
		},
	}

	t.Run("Check repository URL", func(t *testing.T) {
		pacRepo := GeneratePACRepository(component)
		if pacRepo.Spec.URL != repoUrl {
			t.Errorf("Wrong repo URL %s, want %s", pacRepo.Spec.URL, repoUrl)
		}
	})
}

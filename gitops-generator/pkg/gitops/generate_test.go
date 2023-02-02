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

package gitops

import (
	"path/filepath"
	"testing"

	"github.com/mitchellh/go-homedir"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/gitops/prepare"
	"github.com/redhat-developer/gitops-generator/pkg/testutils"
	"github.com/redhat-developer/gitops-generator/pkg/util/ioutils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateTektonBuild(t *testing.T) {
	outputPathBase := "test/"
	fs := ioutils.NewMemoryFilesystem()

	tests := []struct {
		name                 string
		fs                   afero.Afero
		testFolder           string
		component            appstudiov1alpha1.Component
		errors               *testutils.ErrorStack
		want                 []string
		wantErrString        string
		expectFail           bool
		testMessageToDisplay string
	}{
		{
			name:       "Check pipeline as code resources",
			fs:         fs,
			testFolder: "test2",
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
								URL: "https://github.com/user/git-repo.git",
							},
						},
					},
				},
			},
			errors: &testutils.ErrorStack{},
			want: []string{
				kustomizeFileName,
				buildRepositoryFileName,
			},
		},
		{
			name: "Fail build generation because of readonly fs.",
			fs:   ioutils.NewReadOnlyFs(),
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "workspace-name",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "invalid-url-here",
							},
						},
					},
				},
			},
			testMessageToDisplay: "Failure build generation is expected by readonly fs, but seems no error is returned",
			expectFail:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := outputPathBase + tt.testFolder

			if tt.expectFail {
				err := GenerateTektonBuild(outputPath, tt.component, tt.fs, "/", prepare.GitopsConfig{})
				if err == nil {
					t.Errorf(tt.testMessageToDisplay)
				}
			} else {
				if err := GenerateTektonBuild(outputPath, tt.component, tt.fs, "/", prepare.GitopsConfig{}); err != nil {
					t.Errorf("Failed to generate build gitops resources. Cause: %v", err)
				}
			}

			// Ensure that needed resources generated
			path, err := homedir.Expand(outputPath)
			testutils.AssertNoError(t, err)

			for _, item := range tt.want {
				exist, err := tt.fs.Exists(filepath.Join(path, tt.component.Name, "/components/", tt.component.Name, "/base/.tekton/", item))
				testutils.AssertNoError(t, err)
				assert.True(t, exist, "Expected file %s missing in gitops", item)
			}
		})
	}
}

func TestGetAndSetDefaultImageRepo(t *testing.T) {

	tests := []struct {
		name      string
		imageRepo string
		want      string
	}{
		{
			name: "Get default image repo",
			want: "quay.io/redhat-appstudio/user-workload",
		},
		{
			name:      "Override default image repo",
			imageRepo: "quay.io/myuser/myrepo",
			want:      "quay.io/myuser/myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.imageRepo != "" {
				SetDefaultImageRepo(tt.imageRepo)
			}
			imageRepo := GetDefaultImageRepo()
			if imageRepo != tt.want {
				t.Errorf("TestGetAndSetDefaultImageRepo(): want %v, got %v", tt.want, imageRepo)
			}
		})
	}
}

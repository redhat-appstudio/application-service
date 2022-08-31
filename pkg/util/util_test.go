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

package util

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		want        string
	}{
		{
			name:        "Simple display name, no spaces",
			displayName: "PetClinic",
			want:        "petclinic",
		},
		{
			name:        "Simple display name, with space",
			displayName: "PetClinic App",
			want:        "petclinic-app",
		},
		{
			name:        "Longer display name, multiple spaces",
			displayName: "Pet Clinic Application",
			want:        "pet-clinic-application",
		},
		{
			name:        "Very long display name",
			displayName: "Pet Clinic Application Super Super Long Display name",
			want:        "pet-clinic-application-super-super-long-display-na",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := SanitizeName(tt.displayName)
			// Unexpected error
			if sanitizedName != tt.want {
				t.Errorf("SanitizeName() error: expected %v got %v", tt.want, sanitizedName)
			}
		})
	}
}

func TestProcessGitOpsStatus(t *testing.T) {
	tests := []struct {
		name         string
		gitopsStatus appstudiov1alpha1.GitOpsStatus
		gitToken     string
		wantURL      string
		wantBranch   string
		wantContext  string
		wantErr      bool
	}{
		{
			name: "gitops status processed as expected",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/myrepo",
				Branch:        "notmain",
				Context:       "context",
			},
			gitToken:    "token",
			wantURL:     "https://token@github.com/myrepo",
			wantBranch:  "notmain",
			wantContext: "context",
		},
		{
			name: "gitops url is empty",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "",
			},
			wantErr: true,
		},
		{
			name: "gitops branch and context not set",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/myrepo",
			},
			gitToken:    "token",
			wantURL:     "https://token@github.com/myrepo",
			wantBranch:  "main",
			wantContext: "/",
		},
		{
			name: "gitops url parse err",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "http://foo.com/?foo\nbar",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitopsURL, gitopsBranch, gitopsContext, err := ProcessGitOpsStatus(tt.gitopsStatus, tt.gitToken)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				assert.Equal(t, tt.wantURL, gitopsURL, "should be equal")
				assert.Equal(t, tt.wantBranch, gitopsBranch, "should be equal")
				assert.Equal(t, tt.wantContext, gitopsContext, "should be equal")
			}
		})
	}
}

func TestISExist(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		exist   bool
		wantErr bool
	}{
		{
			name:  "Path Exist",
			path:  "/tmp",
			exist: true,
		},
		{
			name:  "Path Does Not Exist",
			path:  "/pathdoesnotexist",
			exist: false,
		},
		{
			name:    "Error Case",
			path:    "\000x",
			exist:   false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isExist, err := IsExist(tt.path)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if isExist != tt.exist {
				t.Errorf("IsExist; expected %v got %v", tt.exist, isExist)
			}
		})
	}
}

func TestCurlEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name: "Valid Endpoint",
			url:  "https://google.ca",
		},
		{
			name:    "Invalid Endpoint",
			url:     "https://google.ca/somepath",
			wantErr: true,
		},
		{
			name:    "Invalid URL",
			url:     "\000x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := CurlEndpoint(tt.url)
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

func TestCloneRepo(t *testing.T) {
	os.Mkdir("/tmp/alreadyexistingdir", 0755)

	tests := []struct {
		name      string
		clonePath string
		repo      string
		token     string
		wantErr   bool
	}{
		{
			name:      "Clone Successfully",
			clonePath: "/tmp/testspringboot",
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
		},
		{
			name:      "Invalid Repo",
			clonePath: "/tmp/testclone",
			repo:      "https://invalid.url",
			wantErr:   true,
		},
		{
			name:      "Invalid Clone Path",
			clonePath: "\000x",
			wantErr:   true,
		},
		{
			name:      "Clone path, already existing folder",
			clonePath: "/tmp/alreadyexistingdir",
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			wantErr:   false,
		},
		{
			name:      "Invalid token, should err out",
			clonePath: "/tmp/alreadyexistingdir",
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			token:     "fake-token",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneRepo(tt.clonePath, tt.repo, tt.token)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

func TestConvertGitHubURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		revision string
		useAPI   bool
		wantUrl  string
		wantErr  bool
	}{
		{
			name:    "Successfully convert a github url to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name:     "Successfully convert a github url with revision to raw url",
			url:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			revision: "testbranch",
			wantUrl:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch",
		},
		{
			name:    "Successfully convert a github url with a trailing / suffix to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic/",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name:     "Successfully convert a github url with revision and a trailing / suffix to raw url",
			url:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic/",
			revision: "testbranch",
			wantUrl:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch",
		},
		{
			name:    "Successfully convert a github url with .git to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic.git",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name:     "Successfully convert a github url with revision and .git to raw url",
			url:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic.git",
			revision: "testbranch",
			wantUrl:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch",
		},
		{
			name:    "A non github url",
			url:     "https://some.url",
			wantUrl: "https://some.url",
		},
		{
			name:    "A raw github url",
			url:     "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
		},
		{
			name:     "A raw github url with revision",
			url:      "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch/devfile.yaml",
			revision: "testbranch",
			wantUrl:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch/devfile.yaml",
		},
		{
			name:    "A non-main branch github url",
			url:     "https://github.com/devfile/api/tree/2.1.x",
			wantUrl: "https://raw.githubusercontent.com/devfile/api/2.1.x",
		},
		{
			name:    "A non url",
			url:     "\000x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convertedUrl, err := ConvertGitHubURL(tt.url, tt.revision)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if convertedUrl != tt.wantUrl {
				t.Errorf("ConvertGitHubURL; expected %v got %v", tt.wantUrl, convertedUrl)
			}
		})
	}
}

func TestCheckWithRegex(t *testing.T) {
	tests := []struct {
		name      string
		test      string
		pattern   string
		wantMatch bool
	}{
		{
			name:      "matching string",
			test:      "hi-00-HI",
			pattern:   "^[a-z]([-a-z0-9]*[a-z0-9])?",
			wantMatch: true,
		},
		{
			name:      "not a matching string",
			test:      "1-hi",
			pattern:   "^[a-z]([-a-z0-9]*[a-z0-9])?",
			wantMatch: false,
		},
		{
			name:      "bad pattern",
			test:      "hi-00-HI",
			pattern:   "(abc",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch := CheckWithRegex(tt.pattern, tt.test)
			assert.Equal(t, tt.wantMatch, gotMatch, "the values should match")
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "Error message with nothing to be sanitized",
			err:  fmt.Errorf("Unable to create component, some error occurred"),
			want: fmt.Errorf("Unable to create component, some error occurred"),
		},
		{
			name: "Error message with token that needs to be sanitized",
			err:  fmt.Errorf("failed clone repository \"https://ghp_fj3492danj924@github.com/fake/repo\""),
			want: fmt.Errorf("failed clone repository \"https://<TOKEN>@github.com/fake/repo\""),
		},
		{
			name: "Error rror message #2 with token that needs to be sanitized",
			err:  fmt.Errorf("random error message with ghp_faketokensdffjfjfn"),
			want: fmt.Errorf("random error message with <TOKEN>"),
		},
		{
			name: "Error message #3 with token that needs to be sanitized",
			err:  fmt.Errorf("ghp_faketoken"),
			want: fmt.Errorf("<TOKEN>"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedError := SanitizeErrorMessage(tt.err)
			if sanitizedError.Error() != tt.want.Error() {
				t.Errorf("SanitizeName() error: expected %v got %v", tt.want, sanitizedError)
			}
		})
	}
}

func TestGetMappedComponent(t *testing.T) {

	tests := []struct {
		name      string
		component appstudiov1alpha1.Component
		want      gitopsgenv1alpha1.Component
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
			want: gitopsgenv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: gitopsgenv1alpha1.ComponentSpec{
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
					Source: gitopsgenv1alpha1.ComponentSource{
						ComponentSourceUnion: gitopsgenv1alpha1.ComponentSourceUnion{
							GitSource: &gitopsgenv1alpha1.GitSource{
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
			want: gitopsgenv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: gitopsgenv1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest002",
					Source:        gitopsgenv1alpha1.ComponentSource{},
				},
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
			want: gitopsgenv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: gitopsgenv1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest003",
				},
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
			want: gitopsgenv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: gitopsgenv1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest004",
					Source: gitopsgenv1alpha1.ComponentSource{
						ComponentSourceUnion: gitopsgenv1alpha1.ComponentSourceUnion{},
					},
				},
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
			want: gitopsgenv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "testnamespace",
				},
				Spec: gitopsgenv1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest005",
					Source: gitopsgenv1alpha1.ComponentSource{
						ComponentSourceUnion: gitopsgenv1alpha1.ComponentSourceUnion{
							GitSource: &gitopsgenv1alpha1.GitSource{},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedComponent := GetMappedGitOpsComponent(tt.component)
			assert.True(t, tt.want.ObjectMeta.Name == mappedComponent.ObjectMeta.Name, "Expected ObjectMeta.Name: %s, is different than actual: %s", tt.want.ObjectMeta.Name, mappedComponent.ObjectMeta.Name)
			assert.True(t, tt.want.ObjectMeta.Namespace == mappedComponent.ObjectMeta.Namespace, "Expected ObjectMeta.Namespace: %s, is different than actual: %s", tt.want.ObjectMeta.Namespace, mappedComponent.ObjectMeta.Namespace)
			assert.True(t, tt.want.Spec.ComponentName == mappedComponent.Spec.ComponentName, "Expected Spec.ComponentName: %s, is different than actual: %s", tt.want.Spec.ComponentName, mappedComponent.Spec.ComponentName)
			assert.True(t, tt.want.Spec.Application == mappedComponent.Spec.Application, "Expected Spec.Application: %s, is different than actual: %s", tt.want.Spec.Application, mappedComponent.Spec.Application)
			assert.True(t, tt.want.Spec.Secret == mappedComponent.Spec.Secret, "Expected Spec.Secret: %s, is different than actual: %s", tt.want.Spec.Secret, mappedComponent.Spec.Secret)
			assert.True(t, reflect.DeepEqual(tt.want.Spec.Resources, mappedComponent.Spec.Resources), "Expected Spec.Resources: %s, is different than actual: %s", tt.want.Spec.Resources, mappedComponent.Spec.Resources)
			assert.True(t, tt.want.Spec.Route == mappedComponent.Spec.Route, "Expected Spec.Route: %s, is different than actual: %s", tt.want.Spec.Route, mappedComponent.Spec.Route)
			assert.True(t, reflect.DeepEqual(tt.want.Spec.Env, mappedComponent.Spec.Env), "Expected Spec.Env: %s, is different than actual: %s", tt.want.Spec.Env, mappedComponent.Spec.Env)
			assert.True(t, tt.want.Spec.ContainerImage == mappedComponent.Spec.ContainerImage, "Expected Spec.ContainerImage: %s, is different than actual: %s", tt.want.Spec.ContainerImage, mappedComponent.Spec.ContainerImage)
			assert.True(t, tt.want.Spec.SkipGitOpsResourceGeneration == mappedComponent.Spec.SkipGitOpsResourceGeneration, "Expected Spec.SkipGitOpsResourceGeneration: %s, is different than actual: %s", tt.want.Spec.SkipGitOpsResourceGeneration, mappedComponent.Spec.SkipGitOpsResourceGeneration)

			if tt.want.Spec.Source.ComponentSourceUnion.GitSource != nil {
				assert.True(t, tt.want.Spec.Source.ComponentSourceUnion.GitSource.URL == mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.URL, "Expected GitSource URL: %s, is different than actual: %s", tt.want.Spec.Source.ComponentSourceUnion.GitSource.URL, mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.URL)
				assert.True(t, tt.want.Spec.Source.ComponentSourceUnion.GitSource.Revision == mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.Revision, "Expected GitSource Revision: %s, is different than actual: %s", tt.want.Spec.Source.ComponentSourceUnion.GitSource.Revision, mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.Revision)
				assert.True(t, tt.want.Spec.Source.ComponentSourceUnion.GitSource.Context == mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.Context, "Expected GitSource Context: %s, is different than actual: %s", tt.want.Spec.Source.ComponentSourceUnion.GitSource.Context, mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.Context)
				assert.True(t, tt.want.Spec.Source.ComponentSourceUnion.GitSource.DevfileURL == mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.DevfileURL, "Expected GitSource DevfileURL: %s, is different than actual: %s", tt.want.Spec.Source.ComponentSourceUnion.GitSource.DevfileURL, mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.DevfileURL)
				assert.True(t, tt.want.Spec.Source.ComponentSourceUnion.GitSource.DockerfileURL == mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.DockerfileURL, "Expected GitSource DockerfileURL: %s, is different than actual: %s", tt.want.Spec.Source.ComponentSourceUnion.GitSource.DockerfileURL, mappedComponent.Spec.Source.ComponentSourceUnion.GitSource.DockerfileURL)
			}
		})
	}
}

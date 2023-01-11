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

package pkg

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	indexSchema "github.com/devfile/registry-support/index/generator/schema"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
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
			repo:      "https://github.com/yangcao77/multi-components-private/",
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
		context  string
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
			context: "./",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name:    "Successfully convert a github url with a context to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic/",
			context: "testfolder",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/testfolder",
		},
		{
			name:    "Successfully convert a github url with a context with a prefix / to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic/",
			context: "/testfolder",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/testfolder",
		},
		{
			name:     "Successfully convert a github url with revision and a trailing / suffix and a context to raw url",
			url:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic/",
			revision: "testbranch",
			context:  "testfolder",
			wantUrl:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch/testfolder",
		},
		{
			name:    "Successfully convert a github url with .git to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic.git",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name:     "Successfully convert a github url with revision and .git and a context with prefix / to raw url",
			url:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic.git",
			revision: "testbranch",
			context:  "/testfolder",
			wantUrl:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch/testfolder",
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
			convertedUrl, err := ConvertGitHubURL(tt.url, tt.revision, tt.context)
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

func TestGetRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
		lower  bool
	}{
		{
			name:   "all lower case string",
			length: 5,
			lower:  true,
		},
		{
			name:   "contain upper case string",
			length: 10,
			lower:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString := GetRandomString(tt.length, tt.lower)
			assert.Equal(t, tt.length, len(gotString), "the values should match")

			if tt.lower == true {
				assert.Equal(t, strings.ToLower(gotString), gotString, "the values should match")
			}

			gotString2 := GetRandomString(tt.length, tt.lower)
			assert.NotEqual(t, gotString, gotString2, "the two random string should not be the same")
		})
	}
}

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

func TestGetContext(t *testing.T) {

	localpath := "/tmp/path/to/a/dir"

	tests := []struct {
		name         string
		currentLevel int
		wantContext  string
	}{
		{
			name:         "1 level",
			currentLevel: 1,
			wantContext:  "dir",
		},
		{
			name:         "2 levels",
			currentLevel: 2,
			wantContext:  "a/dir",
		},
		{
			name:         "0 levels",
			currentLevel: 0,
			wantContext:  "./",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			context := getContext(localpath, tt.currentLevel)
			if tt.wantContext != context {
				t.Errorf("expected %s got %s", tt.wantContext, context)
			}
		})
	}
}

func TestUpdateGitLink(t *testing.T) {

	tests := []struct {
		name     string
		repo     string
		context  string
		wantLink string
		wantErr  bool
	}{
		{
			name:     "context has no http",
			repo:     "https://github.com/maysunfaisal/multi-components-dockerfile/",
			context:  "devfile-sample-java-springboot-basic/docker/Dockerfile",
			wantLink: "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile",
		},
		{
			name:     "context has http",
			repo:     "https://github.com/maysunfaisal/multi-components-dockerfile/",
			context:  "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile",
			wantLink: "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile",
		},
		{
			name:    "err case",
			repo:    "\000x",
			context: "test/dir",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotLink, err := UpdateGitLink(tt.repo, "", tt.context)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if gotLink != tt.wantLink {
				t.Errorf("Expected: %+v, Got: %+v", tt.wantLink, gotLink)
			}

		})
	}
}

func TestGetAlizerDevfileTypes(t *testing.T) {
	const serverIP = "127.0.0.1:9080"

	sampleFilteredIndex := []indexSchema.Schema{
		{
			Name:        "sampleindex1",
			ProjectType: "project1",
			Language:    "language1",
		},
		{
			Name:        "sampleindex2",
			ProjectType: "project2",
			Language:    "language2",
		},
	}

	stackFilteredIndex := []indexSchema.Schema{
		{
			Name: "stackindex1",
		},
		{
			Name: "stackindex2",
		},
	}

	notFilteredIndex := []indexSchema.Schema{
		{
			Name: "index1",
		},
		{
			Name: "index2",
		},
	}

	// Mocking the registry REST endpoints on a very basic level
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var data []indexSchema.Schema
		var err error

		if r.URL.Path == "/index/sample" {
			data = sampleFilteredIndex
		} else if r.URL.Path == "/index/stack" || r.URL.Path == "/index" {
			data = stackFilteredIndex
		} else if r.URL.Path == "/index/all" {
			data = notFilteredIndex
		}

		bytes, err := json.MarshalIndent(&data, "", "  ")
		if err != nil {
			t.Errorf("Unexpected error while doing json marshal: %v", err)
			return
		}

		_, err = w.Write(bytes)
		if err != nil {
			t.Errorf("Unexpected error while writing data: %v", err)
		}
	}))
	// create a listener with the desired port.
	l, err := net.Listen("tcp", serverIP)
	if err != nil {
		t.Errorf("Unexpected error while creating listener: %v", err)
		return
	}

	// NewUnstartedServer creates a listener. Close that listener and replace
	// with the one we created.
	testServer.Listener.Close()
	testServer.Listener = l

	testServer.Start()
	defer testServer.Close()

	tests := []struct {
		name      string
		url       string
		wantTypes []recognizer.DevFileType
		wantErr   bool
	}{
		{
			name: "Get the Sample Devfile Types",
			url:  "http://" + serverIP,
			wantTypes: []recognizer.DevFileType{
				{
					Name:        "sampleindex1",
					ProjectType: "project1",
					Language:    "language1",
				},
				{
					Name:        "sampleindex2",
					ProjectType: "project2",
					Language:    "language2",
				},
			},
		},
		{
			name:    "Not a URL",
			url:     serverIP,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types, err := getAlizerDevfileTypes(tt.url)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if !reflect.DeepEqual(types, tt.wantTypes) {
				t.Errorf("Expected: %+v, \nGot: %+v", tt.wantTypes, types)
			}
		})
	}
}

func TestGetRepoFromRegistry(t *testing.T) {
	const serverIP = "127.0.0.1:9080"

	index := []indexSchema.Schema{
		{
			Name:        "index1",
			ProjectType: "project1",
			Language:    "language1",
			Git: &indexSchema.Git{
				Remotes: map[string]string{
					"origin": "repo",
				},
			},
		},
		{
			Name:        "index2",
			ProjectType: "project2",
			Language:    "language2",
		},
	}

	// Mocking the registry REST endpoints on a very basic level
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var data []indexSchema.Schema
		var err error

		if r.URL.Path == "/index/sample" {
			data = index
		} else if r.URL.Path == "/index/all" {
			data = index
		}

		bytes, err := json.MarshalIndent(&data, "", "  ")
		if err != nil {
			t.Errorf("Unexpected error while doing json marshal: %v", err)
			return
		}

		_, err = w.Write(bytes)
		if err != nil {
			t.Errorf("Unexpected error while writing data: %v", err)
		}
	}))
	// create a listener with the desired port.
	l, err := net.Listen("tcp", serverIP)
	if err != nil {
		t.Errorf("Unexpected error while creating listener: %v", err)
		return
	}

	// NewUnstartedServer creates a listener. Close that listener and replace
	// with the one we created.
	testServer.Listener.Close()
	testServer.Listener = l

	testServer.Start()
	defer testServer.Close()

	tests := []struct {
		name       string
		url        string
		sampleName string
		wantURL    string
		wantErr    bool
	}{
		{
			name:       "Get the Repo URL from the sample",
			url:        "http://" + serverIP,
			sampleName: "index1",
			wantURL:    "repo",
		},
		{
			name:       "Sample does not have a Repo URL",
			url:        "http://" + serverIP,
			sampleName: "index2",
			wantErr:    true,
		},
		{
			name:    "Not a URL",
			url:     serverIP,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, err := GetRepoFromRegistry(tt.sampleName, tt.url)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if !reflect.DeepEqual(gotURL, tt.wantURL) {
				t.Errorf("Expected: %+v, \nGot: %+v", tt.wantURL, gotURL)
			}
		})
	}
}

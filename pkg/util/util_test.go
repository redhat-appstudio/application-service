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

package util

import (
	"os"
	"strings"
	"testing"

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

func TestValidateEndpoint(t *testing.T) {
	invalidEndpoint := "failed to get the url"
	parseFail := "failed to parse the url"

	tests := []struct {
		name    string
		url     string
		wantErr *string
	}{
		{
			name: "Valid Endpoint",
			url:  "https://google.ca",
		},
		{
			name: "Valid private repo",
			url:  "https://github.com/yangcao77/multi-components-private",
		},
		{
			name:    "Invalid Endpoint",
			url:     "protocal://google.ca/somepath",
			wantErr: &invalidEndpoint,
		},
		{
			name:    "Invalid URL failed to be parsed",
			url:     "\000x",
			wantErr: &parseFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpoint(tt.url)
			if tt.wantErr != nil && (err == nil) {
				t.Error("wanted error but got nil")
				return
			} else if tt.wantErr == nil && err != nil {
				t.Errorf("got unexpected error %v", err)
				return
			}
			if tt.wantErr != nil {
				assert.Regexp(t, *tt.wantErr, err.Error(), "TestValidateEndpoint: Error message does not match")
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
		revision  string
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
		{
			name:      "Clone Successfully - branch specified as revision",
			clonePath: "/tmp/testspringboot",
			repo:      "https://github.com/devfile-resources/node-express-hello-no-devfile",
			revision:  "testbranch",
		},
		{
			name:      "Clone Successfully - commit specified as revision",
			clonePath: "/tmp/nodeexpressrevision",
			repo:      "https://github.com/devfile-resources/node-express-hello-no-devfile",
			revision:  "22d213a42091199bc1f85a8eac60a5ff82371df3",
		},
		{
			name:      "Invalid revision, should err out",
			clonePath: "/tmp/nodeexpressrevisioninvalidrevision",
			repo:      "https://github.com/devfile-resources/node-express-hello-no-devfile",
			revision:  "fasdfasdfasdfdsklafj2w23",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneRepo(tt.clonePath, tt.repo, tt.revision, tt.token)
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

func TestGetIntValue(t *testing.T) {

	value := 7

	tests := []struct {
		name      string
		replica   *int
		wantValue int
		wantErr   bool
	}{
		{
			name:      "Unset value, expect default 0",
			replica:   nil,
			wantValue: 0,
		},
		{
			name:      "set value, expect set number",
			replica:   &value,
			wantValue: 7,
		},
	}

	for _, tt := range tests {
		val := GetIntValue(tt.replica)
		assert.True(t, val == tt.wantValue, "Expected int value %d got %d", tt.wantValue, val)
	}
}

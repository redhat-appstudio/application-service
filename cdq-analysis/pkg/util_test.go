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

package pkg

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	indexSchema "github.com/devfile/registry-support/index/generator/schema"
	"github.com/redhat-developer/alizer/go/pkg/apis/model"
)

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

func TestGetBranchFromRepo(t *testing.T) {
	os.Mkdir("/tmp/alreadyexistingdir", 0755)

	tests := []struct {
		name      string
		clonePath string
		repo      string
		revision  string
		token     string
		wantErr   bool
		want      string
	}{
		{
			name:      "Detect Successfully",
			clonePath: "/tmp/testspringbootclone",
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			want:      "main",
		},
		{
			name:      "Detect alternate branch Successfully",
			clonePath: "/tmp/testspringbootclonealt",
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			revision:  "testbranch",
			want:      "testbranch",
		},
		{
			name:      "Repo not exist",
			clonePath: "FDSFSDFSDFSDFjsdklfjsdklfjs",
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.RemoveAll(tt.clonePath)
			if tt.name != "Repo not exist" {
				CloneRepo(tt.clonePath, tt.repo, tt.revision, tt.token)
			}

			branch, err := GetBranchFromRepo(tt.clonePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGetBranchFromRepo() unexpected error: %v", err)
			}
			if err != nil {
				if branch != tt.want {
					t.Errorf("TestGetBranchFromRepo() unexpected branch, expected %v got %v", tt.want, branch)
				}
			}
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
		wantTypes []model.DevFileType
		wantErr   bool
	}{
		{
			name: "Get the Sample Devfile Types",
			url:  "http://" + serverIP,
			wantTypes: []model.DevFileType{
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

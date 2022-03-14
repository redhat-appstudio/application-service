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
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	indexSchema "github.com/devfile/registry-support/index/generator/schema"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
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

func TestDownloadDevfile(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name: "Curl devfile.yaml",
			url:  "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name: "Curl .devfile.yaml",
			url:  "https://raw.githubusercontent.com/maysunfaisal/hiddendevfile/main",
		},
		{
			name: "Curl .devfile/devfile.yaml",
			url:  "https://raw.githubusercontent.com/maysunfaisal/hiddendirdevfile/main",
		},
		{
			name: "Curl .devfile/.devfile.yaml",
			url:  "https://raw.githubusercontent.com/maysunfaisal/hiddendirhiddendevfile/main",
		},
		{
			name:    "Cannot curl for a devfile",
			url:     "https://github.com/octocat/Hello-World",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := DownloadDevfile(tt.url)
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
			clonePath: "/tmp/testclone",
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
		name    string
		url     string
		useAPI  bool
		wantUrl string
		wantErr bool
	}{
		{
			name:    "Successfully convert a github url to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
		},
		{
			name:    "Successfully convert a github url with .git to raw url",
			url:     "https://github.com/devfile-samples/devfile-sample-java-springboot-basic.git",
			wantUrl: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main",
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
			convertedUrl, err := ConvertGitHubURL(tt.url)
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

func TestReadDevfilesFromRepo(t *testing.T) {
	tests := []struct {
		name                   string
		clonePath              string
		depth                  int
		repo                   string
		token                  string
		wantErr                bool
		expectedDevfileContext []string
	}{
		{
			name:      "Should return 0 devfiles as this is not a multi comp devfile",
			clonePath: "/tmp/testclone",
			depth:     1,
			repo:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
			wantErr:   true,
		},
		{
			name:                   "Should return 1 devfiles as this is a multi comp devfile",
			clonePath:              "/tmp/testclone",
			depth:                  1,
			repo:                   "https://github.com/maysunfaisal/multi-components-deep",
			expectedDevfileContext: []string{"devfile-sample-java-springboot-basic", "python"},
		},
		// TODO - maysunfaisal
		// Commenting out this test case, we hard code our depth to 1 for CDQ
		// But there seems to a gap in the logic if we extend past depth 1 and discovering devfile logic
		// Revisit post M4
		// {
		// 	name:                   "Should return 2 devfiles as this is a multi comp devfile",
		// 	clonePath:              "/tmp/testclone",
		// 	depth:                  2,
		// 	repo:                   "https://github.com/maysunfaisal/multi-components-deep",
		// 	expectedDevfileContext: []string{"devfile-sample-java-springboot-basic", "python/devfile-sample-python-basic"},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneRepo(tt.clonePath, tt.repo, tt.token)
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				devfileMap, _, err := ReadDevfilesFromRepo(tt.clonePath, tt.depth)
				if tt.wantErr && (err == nil) {
					t.Error("wanted error but got nil")
				} else if !tt.wantErr && err != nil {
					t.Errorf("got unexpected error %v", err)
				} else {
					for actualContext := range devfileMap {
						matched := false
						for _, context := range tt.expectedDevfileContext {
							if context == actualContext {
								matched = true
							}
						}

						if !matched {
							t.Errorf("found devfile at context %v but expected none", actualContext)
						}
					}
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}

func TestGetAlizerDevfileTypes(t *testing.T) {
	const serverIP = "127.0.0.1:8080"

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

func TestAnalyzeAndDetectDevfile(t *testing.T) {

	tests := []struct {
		name                string
		clonePath           string
		repo                string
		wantDevfile         bool
		wantDevfileEndpoint string
		wantErr             bool
	}{
		{
			name:                "Successfully detect a devfile from the registry",
			clonePath:           "/tmp/testclone",
			repo:                "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			wantDevfile:         true,
			wantDevfileEndpoint: "https://registry.stage.devfile.io/devfiles/java-springboot-basic",
		},
		{
			name:      "Cannot detect a devfile for a Go repository",
			clonePath: "/tmp/testclone",
			repo:      "https://github.com/devfile/devworkspace-operator",
			wantErr:   true,
		},
		// {
		// 	name:      "Invalid Path",
		// 	clonePath: "/tmp/testclone",
		// 	repo:      "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
		// 	wantErr:   false,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneRepo(tt.clonePath, tt.repo)
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				path := tt.clonePath
				if tt.name == "Invalid Path" {
					path = ""
				}
				devfileBytes, detectedDevfileEndpoint, err := AnalyzeAndDetectDevfile(path)
				if !tt.wantErr && err != nil {
					t.Errorf("Unexpected err: %+v", err)
				} else if tt.wantErr && err == nil {
					t.Errorf("Expected error but got nil")
				} else if !reflect.DeepEqual(len(devfileBytes) > 0, tt.wantDevfile) {
					t.Errorf("Expected devfile: %+v, \nGot: %+v", tt.wantDevfile, len(devfileBytes) > 0)
				} else if !reflect.DeepEqual(detectedDevfileEndpoint, tt.wantDevfileEndpoint) {
					t.Errorf("Expected devfile endpoint: %+v, \nGot: %+v", tt.wantDevfileEndpoint, detectedDevfileEndpoint)
				}
			}
			os.RemoveAll(tt.clonePath)
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

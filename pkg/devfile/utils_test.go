//
// Copyright 2022-2023 Red Hat, Inc.
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

package devfile

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	indexSchema "github.com/devfile/registry-support/index/generator/schema"
	"github.com/redhat-developer/alizer/go/pkg/apis/model"
)

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

func TestGetIngressHostName(t *testing.T) {

	tests := []struct {
		name          string
		componentName string
		namespace     string
		ingressDomain string
		wantHostName  string
		wantErr       bool
	}{
		{
			name:          "all string present",
			componentName: "my-component",
			namespace:     "test",
			ingressDomain: "domain.example.com",
			wantHostName:  "my-component-test.domain.example.com",
		},
		{
			name:          "Capitalized component name should be ok",
			componentName: "my-Component",
			namespace:     "test",
			ingressDomain: "domain.example.com",
			wantHostName:  "my-Component-test.domain.example.com",
		},
		{
			name:          "invalid char in string",
			componentName: "&",
			namespace:     "$",
			ingressDomain: "$",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotHostName, err := GetIngressHostName(tt.componentName, tt.namespace, tt.ingressDomain)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if !reflect.DeepEqual(tt.wantHostName, gotHostName) {
				t.Errorf("Expected: %+v, \nGot: %+v", tt.wantHostName, gotHostName)
			}
		})
	}
}

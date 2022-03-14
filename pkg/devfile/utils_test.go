package devfile

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	indexSchema "github.com/devfile/registry-support/index/generator/schema"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
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

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
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestIsExisting(t *testing.T) {
	fs := NewFilesystem()
	inmemoryFs := NewMemoryFilesystem()
	readOnlyFs := NewReadOnlyFs()
	dirName := "/tmp/test-dir"
	fileName := "/tmp/test-file"
	secondFile := "/tmp/test-two"

	// Make sure at least one file and one dir exists in each file system for testing
	fs.Create(fileName)
	// fs.Mkdir(dirName, 0755)
	inmemoryFs.Create(fileName)
	inmemoryFs.Mkdir(dirName, 0755)

	tests := []struct {
		name          string
		path          string
		want          bool
		wantErrString string
		fs            afero.Afero
	}{
		{
			name:          "Simple file does not exist, inmemory fs",
			path:          secondFile,
			want:          false,
			wantErrString: "open /tmp/test-two: file does not exist",
			fs:            inmemoryFs,
		},
		{
			name:          "File exists, inmemory fs",
			path:          fileName,
			want:          true,
			wantErrString: "\"test-file\": File already exists at /tmp/test-file",
			fs:            inmemoryFs,
		},
		{
			name:          "Dir already exists, inmemory fs",
			path:          dirName,
			want:          true,
			wantErrString: "\"test-dir\": Dir already exists at /tmp/test-dir",
			fs:            inmemoryFs,
		},
		{
			name:          "File does not exist, regular fs",
			path:          secondFile,
			want:          false,
			wantErrString: "stat /tmp/test-two: no such file or directory",
			fs:            fs,
		},
		{
			name:          "Dir already exists, readonly fs",
			path:          "/",
			want:          true,
			wantErrString: "\"/\": Dir already exists at /",
			fs:            readOnlyFs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := IsExisting(tt.fs, tt.path)
			if tt.wantErrString != "" {
				if err == nil {
					t.Errorf("TestIsExisting() expected error: %v, got: %v", tt.wantErrString, nil)
				} else if err.Error() != tt.wantErrString {
					t.Errorf("TestIsExisting() expected error: %v, got: %v", tt.wantErrString, err.Error())
				}
			} else if tt.wantErrString == "" && err != nil {
				t.Errorf("TestIsExisting() unexpected error: %v, got: %v", tt.wantErrString, err)
			}

			if exists != tt.want {
				t.Errorf("TestIsExisting() expected: %v, got: %v", tt.want, exists)
			}

		})
	}
}

func TestCreateTempPath(t *testing.T) {
	fs := NewFilesystem()
	inmemoryFs := NewMemoryFilesystem()
	readOnlyFs := NewReadOnlyFs()

	tests := []struct {
		name    string
		fs      afero.Afero
		wantErr bool
	}{
		{
			name: "inmemory fs",
			fs:   inmemoryFs,
		},
		{
			name:    "read only fs",
			fs:      readOnlyFs,
			wantErr: true,
		},
		{
			name: "local fs",
			fs:   fs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := CreateTempPath("TestCreateTempPath", tt.fs)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error: %v", err)
			} else if err == nil {
				if !strings.Contains(path, os.TempDir()) {
					t.Errorf("TestCreateTempPath error: the temp path should be in the OS temp dir")
				}

				if !strings.Contains(path, "TestCreateTempPath") {
					t.Errorf("TestCreateTempPath error: the temp path should contain the prefix")
				}

				if isExist, err := IsExisting(tt.fs, path); isExist {
					assert.NoError(t, tt.fs.RemoveAll(path), "unable to delete the temp path")
				} else if err != nil {
					t.Errorf("TestCreateTempPath unexpected error: %v", err)
				}
			}
		})
	}
}

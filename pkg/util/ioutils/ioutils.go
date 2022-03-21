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

// Originally from https://github.com/redhat-developer/kam/blob/master/pkg/pipelines/ioutils/file_utils.go

package ioutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// NewFilesystem returns a local filesystem based afero FS implementation.
func NewFilesystem() afero.Afero {
	return afero.Afero{Fs: afero.NewOsFs()}
}

// NewMemoryFilesystem returns an in-memory afero FS implementation.
func NewMemoryFilesystem() afero.Afero {
	return afero.Afero{Fs: afero.NewMemMapFs()}
}

// NewReadOnlyFs returns a read-only file system
func NewReadOnlyFs() afero.Afero {
	return afero.Afero{Fs: afero.NewReadOnlyFs(afero.NewOsFs())}
}

// IsExisting returns bool whether path exists
func IsExisting(fs afero.Fs, path string) (bool, error) {
	fileInfo, err := fs.Stat(path)
	if err != nil {
		return false, err
	}
	if fileInfo.IsDir() {
		return true, fmt.Errorf("%q: Dir already exists at %s", filepath.Base(path), path)
	}
	return true, fmt.Errorf("%q: File already exists at %s", filepath.Base(path), path)
}

// CreateTempPath creates a temp path with the prefix using the Afero FS
func CreateTempPath(prefix string, appFs afero.Afero) (string, error) {
	return appFs.TempDir(os.TempDir(), prefix)
}

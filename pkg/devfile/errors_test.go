//
// Copyright 2022 Red Hat, Inc.
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoDevfileFoundErr(t *testing.T) {

	tests := []struct {
		name          string
		args          NoDevfileFound
		wantErrString string
	}{
		{
			name: "No Devfile Found at location",
			args: NoDevfileFound{
				Location: "/path",
			},
			wantErrString: "unable to find devfile in the specified location /path",
		},
		{
			name: "No Devfile Found at location due to an err",
			args: NoDevfileFound{
				Location: "/path",
				Err:      fmt.Errorf("a dummy err"),
			},
			wantErrString: "unable to find devfile in the specified location /path due to a dummy err",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errString := tt.args.Error()
			assert.Equal(t, tt.wantErrString, errString, "the err string should be equal")
		})
	}
}

func TestNoDockerfileFoundErr(t *testing.T) {

	tests := []struct {
		name          string
		args          NoDockerfileFound
		wantErrString string
	}{
		{
			name: "No Dockerfile Found at location",
			args: NoDockerfileFound{
				Location: "/path",
			},
			wantErrString: "unable to find Dockerfile in the specified location /path",
		},
		{
			name: "No Dockerfile Found at location due to an err",
			args: NoDockerfileFound{
				Location: "/path",
				Err:      fmt.Errorf("a dummy err"),
			},
			wantErrString: "unable to find Dockerfile in the specified location /path due to a dummy err",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errString := tt.args.Error()
			assert.Equal(t, tt.wantErrString, errString, "the err string should be equal")
		})
	}
}

func TestNoFileFoundErr(t *testing.T) {

	tests := []struct {
		name          string
		args          NoFileFound
		wantErrString string
	}{
		{
			name: "No file Found at location",
			args: NoFileFound{
				Location: "/path",
			},
			wantErrString: "unable to find file in the specified location /path",
		},
		{
			name: "No file Found at location due to an err",
			args: NoFileFound{
				Location: "/path",
				Err:      fmt.Errorf("a dummy err"),
			},
			wantErrString: "unable to find file in the specified location /path due to a dummy err",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errString := tt.args.Error()
			assert.Equal(t, tt.wantErrString, errString, "the err string should be equal")
		})
	}
}

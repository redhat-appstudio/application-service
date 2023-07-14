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
	"reflect"
	"testing"
)

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

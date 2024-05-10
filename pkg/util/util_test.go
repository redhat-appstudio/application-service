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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrInList(t *testing.T) {
	tests := []struct {
		name string
		str  string
		list []string
		want bool
	}{
		{
			name: "str not in list",
			str:  "test",
			list: []string{"some", "words"},
			want: false,
		},
		{
			name: "str in list",
			str:  "test",
			list: []string{"some", "test", "words"},
			want: true,
		},
	}

	for _, tt := range tests {
		val := StrInList(tt.str, tt.list)
		assert.True(t, val == tt.want, "Expected bool value %v got %v", tt.want, val)
	}
}

func TestRemoveStrFromList(t *testing.T) {
	tests := []struct {
		name string
		str  string
		list []string
		want []string
	}{
		{
			name: "single string in list",
			str:  "test",
			list: []string{"some", "test", "words"},
			want: []string{"some", "words"},
		},
		{
			name: "string not in list",
			str:  "test",
			list: []string{"some", "words"},
			want: []string{"some", "words"},
		},
		{
			name: "multiple occurence of string in list",
			str:  "test",
			list: []string{"some", "test", "words", "test", "again"},
			want: []string{"some", "words", "test", "again"},
		},
	}

	for _, tt := range tests {
		strList := RemoveStrFromList(tt.str, tt.list)
		if len(strList) != len(tt.want) {
			t.Errorf("TestRemoveStrFromList(): unexpected error. expected string list %v, got %v", tt.want, strList)
		}
		for i := range strList {
			if strList[i] != tt.want[i] {
				t.Errorf("TestRemoveStrFromList(): unexpected error. expected string %v at index %v, got %v", tt.want[i], i, strList[i])
			}
		}
	}
}

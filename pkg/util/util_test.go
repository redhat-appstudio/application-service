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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEndpoint(t *testing.T) {
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
			url:  "https://github.com/devfile-resources/multi-components-private",
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

func TestValidateGithubURL(t *testing.T) {
	tests := []struct {
		name         string
		sourceGitURL string
		wantErr      bool
	}{
		{
			name:         "Valid github url",
			sourceGitURL: "https://github.com/devfile-samples",
			wantErr:      false,
		},
		{
			name:         "Invalid url",
			sourceGitURL: "afgae devfile",
			wantErr:      true,
		},
		{
			name:         "Not github url",
			sourceGitURL: "https://gitlab.com/devfile-samples",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGithubURL(tt.sourceGitURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestValidateGithubURL() unexpected error: %v", err)
			}
		})
	}
}

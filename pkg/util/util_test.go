//
// Copyright 2021 Red Hat, Inc.
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
			sanitizedName := SanitizeDisplayName(tt.displayName)
			// Unexpected error
			if sanitizedName != tt.want {
				t.Errorf("TestSanitizeDisplayName() error: expected %v got %v", tt.want, sanitizedName)
			}
		})
	}
}

func TestGenerateNewRepositoryName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		namespace   string
		want        string
	}{
		{
			name:        "Simple display name, no spaces",
			displayName: "PetClinic",
			namespace:   "default",
			want:        "petclinic-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := SanitizeDisplayName(tt.displayName)
			generatedRepo := GenerateNewRepositoryName(tt.displayName, tt.namespace)

			if !strings.Contains(generatedRepo, sanitizedName) {
				t.Errorf("TestSanitizeDisplayName() error: expected %v got %v", tt.want, sanitizedName)
			}
		})
	}
}

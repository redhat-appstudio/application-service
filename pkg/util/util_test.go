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

func TestGenerateNewRepository(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := SanitizeDisplayName(tt.displayName)
			generatedRepo := GenerateNewRepository(tt.displayName)

			if !strings.Contains(generatedRepo, sanitizedName) {
				t.Errorf("TestSanitizeDisplayName() error: expected %v got %v", tt.want, sanitizedName)
			}
		})
	}
}

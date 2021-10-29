package util

import (
	"strings"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

const AppStudioDataOrg = "https://github.com/redhat-appstudio-appdata/"

func SanitizeDisplayName(displayName string) string {
	sanitizedName := strings.ToLower(strings.Replace(displayName, " ", "-", -1))
	if len(sanitizedName) > 50 {
		sanitizedName = sanitizedName[0:50]
	}
	return sanitizedName
}

func GenerateNewRepository(displayName string) string {
	sanitizedName := SanitizeDisplayName(displayName)

	repoName := sanitizedName + "-" + gofakeit.Verb() + "-" + gofakeit.Noun()
	repository := AppStudioDataOrg + repoName
	return repository
}

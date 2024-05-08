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
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
)

var RevisionHistoryLimit = int32(0)

// GetIntValue returns the value of an int pointer, with the default of 0 if nil
func GetIntValue(intPtr *int) int {
	if intPtr != nil {
		return *intPtr
	}

	return 0
}

// ValidateEndpoint validates if the endpoint url can be parsed and if it has a host and a scheme
func ValidateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse the url: %v, err: %v", endpoint, err)
	}

	if len(u.Host) == 0 || len(u.Scheme) == 0 {
		return fmt.Errorf("url %v is invalid", endpoint)
	}

	return nil
}

// CheckWithRegex checks if a name matches the pattern.
// If a pattern fails to compile, it returns false
func CheckWithRegex(pattern, name string) bool {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return reg.MatchString(name)
}

const schemaBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// GetRandomString returns a random string which is n characters long.
// If lower is set to true a lower case string is returned.
func GetRandomString(n int, lower bool) string {
	b := make([]byte, n)
	for i := range b {
		/* #nosec G404 -- not used for cryptographic purposes*/
		b[i] = schemaBytes[rand.Intn(len(schemaBytes)-1)]
	}
	randomString := string(b)
	if lower {
		randomString = strings.ToLower(randomString)
	}
	return randomString
}

// StrInList returns true if the given string is present in strList
func StrInList(str string, strList []string) bool {
	for _, val := range strList {
		if str == val {
			return true
		}
	}
	return false
}

// RemoveStrFromList removes the first occurence of str from the slice strList
func RemoveStrFromList(str string, strList []string) []string {
	for i, v := range strList {
		if v == str {
			return append(strList[:i], strList[i+1:]...)
		}
	}
	return strList
}

// ValidateGithubURL checks if the given url includes github in hostname
// In case of invalid url (not able to parse / not github) returns an error.
func ValidateGithubURL(URL string) error {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return err
	}

	if strings.Contains(parsedURL.Host, "github") {
		return nil
	}
	return fmt.Errorf("source git url %v is not from github", URL)
}

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
	"strings"

	"github.com/redhat-developer/alizer/go/pkg/apis/language"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

type MockAlizerClient struct {
}

// Analyze is a wrapper call to Alizer's Analyze()
func (a MockAlizerClient) Analyze(path string) ([]language.Language, error) {
	if strings.Contains(path, "/error/Analyze") {
		return nil, fmt.Errorf("dummy err")
	}
	languages := []language.Language{
		{
			Name:              "nodejs-basic",
			UsageInPercentage: 60.4,
			CanBeComponent:    true,
		},
		{
			Name:              "java",
			UsageInPercentage: 22.4,
			CanBeComponent:    true,
		},
	}

	return languages, nil
}

// SelectDevFileFromTypes is a wrapper call to Alizer's SelectDevFileFromTypes()
func (a MockAlizerClient) SelectDevFileFromTypes(path string, devFileTypes []recognizer.DevFileType) (recognizer.DevFileType, error) {
	if strings.Contains(path, "/error/SelectDevFileFromTypes") {
		return recognizer.DevFileType{}, fmt.Errorf("dummy err")
	} else if strings.Contains(path, "/java-springboot-basic") {
		return recognizer.DevFileType{
			Name: "java-springboot-basic",
		}, nil
	} else if strings.Contains(path, "/error/devfileendpoint") {
		return recognizer.DevFileType{
			Name: "fake",
		}, nil
	}

	return recognizer.DevFileType{}, nil
}

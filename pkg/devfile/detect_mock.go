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

// DetectComponents is a wrapper call to Alizer's DetectComponents()
func (a MockAlizerClient) DetectComponents(path string) ([]recognizer.Component, error) {
	if strings.Contains(path, "errorAnalyze") {
		return nil, fmt.Errorf("dummy DetectComponents err")
	} else if strings.Contains(path, "devfile-sample-nodejs-basic") {
		return []recognizer.Component{
			{
				Path: path,
				Languages: []language.Language{
					{
						Name:              "nodejs",
						UsageInPercentage: 60.4,
						CanBeComponent:    true,
					},
				},
			},
		}, nil
	} else if !strings.Contains(path, "springboot") && !strings.Contains(path, "python") {
		return nil, nil
	}

	return []recognizer.Component{
		{
			Path: path,
			Languages: []language.Language{
				{
					Name:              "springboot",
					UsageInPercentage: 60.4,
					CanBeComponent:    true,
				},
				{
					Name:              "python",
					UsageInPercentage: 22.4,
					CanBeComponent:    true,
				},
			},
		},
	}, nil
}

// SelectDevFileFromTypes is a wrapper call to Alizer's SelectDevFileFromTypes()
func (a MockAlizerClient) SelectDevFileFromTypes(path string, devFileTypes []recognizer.DevFileType) (recognizer.DevFileType, error) {
	if strings.Contains(path, "/errorSelectDevFileFromTypes") {
		return recognizer.DevFileType{}, fmt.Errorf("dummy SelectDevFileFromTypes err")
	} else if strings.Contains(path, "/error/devfileendpoint") {
		return recognizer.DevFileType{
			Name: "fake",
		}, nil
	} else if strings.Contains(path, "java-springboot-basic") || strings.Contains(path, "springboot") {
		return recognizer.DevFileType{
			Name: "java-springboot-basic",
		}, nil
	} else if strings.Contains(path, "devfile-sample-nodejs-basic") {
		return recognizer.DevFileType{
			Name: "nodejs-basic",
		}, nil
	} else if strings.Contains(path, "python-basic") {
		return recognizer.DevFileType{
			Name: "python-basic",
		}, nil
	}

	return recognizer.DevFileType{}, nil
}

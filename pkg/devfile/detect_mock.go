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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/redhat-developer/alizer/go/pkg/apis/model"
)

type MockAlizerClient struct {
}

// DetectComponents is a wrapper call to Alizer's DetectComponents()
func (a MockAlizerClient) DetectComponents(path string) ([]model.Component, error) {
	if strings.Contains(path, "errorAnalyze") {
		return nil, fmt.Errorf("dummy DetectComponents err")
	} else if strings.Contains(path, "empty") {
		return []model.Component{}, nil
	} else if strings.Contains(path, "devfile-sample-nodejs-basic") {
		return []model.Component{
			{
				Path: path,
				Languages: []model.Language{
					{
						Name:           "nodejs",
						Weight:         60.4,
						CanBeComponent: true,
					},
				},
				Ports: []int{8080},
			},
		}, nil
	} else if strings.Contains(path, "nodejs-no-dockerfile") {
		return []model.Component{
			{
				Path: path,
				Languages: []model.Language{
					{
						Name: "JavaScript",
						Aliases: []string{
							"js",
							"node",
							"nodejs",
						},
						Frameworks: []string{
							"Express",
						},
						Tools: []string{
							"NodeJs",
							"Node.js",
						},
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
		}, nil
	} else if strings.Contains(path, "dockerfile-node-sample") {
		return []model.Component{
			{
				Path:  path,
				Ports: []int{5050},
				Languages: []model.Language{
					{
						Name: "JavaScript",
						Aliases: []string{
							"js",
							"node",
							"nodejs",
						},
						Frameworks: []string{
							"Express",
						},
						Tools: []string{
							"NodeJs",
							"Node.js",
						},
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
		}, nil
	} else if strings.Contains(path, "python-src-none") {
		return []model.Component{
			{
				Path: path,
				Languages: []model.Language{
					{
						Name:           "python",
						Weight:         99,
						CanBeComponent: true,
					},
				},
			},
		}, nil
	} else if strings.Contains(path, "python-src-docker") {
		return []model.Component{
			{
				Path: path,
				Languages: []model.Language{
					{
						Name:           "python",
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
		}, nil
	} else if strings.Contains(path, "spring-boot-root-component") {
		return []model.Component{
			{
				Path: path,
				Languages: []model.Language{
					{
						Name: "Java",
						Frameworks: []string{
							"Spring",
						},
						Tools: []string{
							"Maven",
						},
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
			{
				Path: filepath.Join(path, "src/main/resources/static"),
				Languages: []model.Language{
					{
						Name: "JavaScript",
						Aliases: []string{
							"js",
							"node",
							"nodejs",
							"TypeScript",
						},
						Frameworks: []string{
							"Vue",
						},
						Tools: []string{
							"NodeJs",
							"Node.js",
						},
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
		}, nil
	} else if strings.Contains(path, "quality-dashboard") {
		return []model.Component{
			{
				Path: filepath.Join(path, "backend"),
				Languages: []model.Language{
					{
						Name: "Go",
						Aliases: []string{
							"golang",
						},
						Frameworks: []string{
							"Mux",
						},
						Tools: []string{
							"1.18",
						},
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
			{
				Path: filepath.Join(path, "frontend"),
				Languages: []model.Language{
					{
						Name: "Typescript",
						Aliases: []string{
							"ts",
							"Javascript",
						},
						Frameworks: []string{
							"React",
						},
						Tools: []string{
							"NodeJs",
							"Node.js",
						},
						Weight:         100,
						CanBeComponent: true,
					},
				},
			},
		}, nil
	} else if !strings.Contains(path, "springboot") && !strings.Contains(path, "python") {
		return nil, nil
	}

	return []model.Component{
		{
			Path: path,
			Languages: []model.Language{
				{
					Name:           "springboot",
					Weight:         60.4,
					CanBeComponent: true,
				},
				{
					Name:           "python",
					Weight:         22.4,
					CanBeComponent: true,
				},
			},
		},
	}, nil
}

// SelectDevFileFromTypes is a wrapper call to Alizer's SelectDevFileFromTypes()
func (a MockAlizerClient) SelectDevFileFromTypes(path string, devFileTypes []model.DevFileType) (model.DevFileType, error) {
	if strings.Contains(path, "/errorSelectDevFileFromTypes") {
		return model.DevFileType{}, fmt.Errorf("dummy SelectDevFileFromTypes err")
	} else if strings.Contains(path, "/error/devfileendpoint") {
		return model.DevFileType{
			Name: "fake",
		}, nil
	} else if strings.Contains(path, "java-springboot-basic") || strings.Contains(path, "springboot") || strings.Contains(path, "spring-boot-root-component") {
		return model.DevFileType{
			Name: "java-springboot-basic",
		}, nil
	} else if strings.Contains(path, "devfile-sample-nodejs-basic") {
		return model.DevFileType{
			Name: "nodejs-basic",
		}, nil
	} else if strings.Contains(path, "python-basic") || strings.Contains(path, "python-src-none") || strings.Contains(path, "python-src-docker") {
		return model.DevFileType{
			Name: "python-basic",
		}, nil
	} else if strings.Contains(path, "nodejs-no-dockerfile") || strings.Contains(path, "dockerfile-node-sample") {
		return model.DevFileType{
			Name:        "nodejs-basic",
			Language:    "JavaScript",
			ProjectType: "Node.js",
			Tags: []string{
				"Node.js",
				"Express",
				"ubi8",
			},
		}, nil
	}

	return model.DevFileType{}, nil
}

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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponentCreateValidatingWebhook(t *testing.T) {

	tests := []struct {
		name    string
		newComp Component
		err     string
	}{
		{
			name: "component name cannot be created due to bad URL",
			err:  "invalid URI for request",
			newComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: ComponentSource{
						ComponentSourceUnion: ComponentSourceUnion{
							GitSource: &GitSource{
								URL: "badurl",
							},
						},
					},
				},
			},
		},
		{
			name: "component needs to have one source specified",
			err:  "git source or an image source must be specified",
			newComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: ComponentSource{
						ComponentSourceUnion: ComponentSourceUnion{
							GitSource: &GitSource{},
						},
					},
				},
			},
		},
		{
			name: "valid component with git src",
			newComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: ComponentSource{
						ComponentSourceUnion: ComponentSourceUnion{
							GitSource: &GitSource{
								URL: "http://url",
							},
						},
					},
				},
			},
		},
		{
			name: "valid component with container image",
			newComp: Component{
				Spec: ComponentSpec{
					ComponentName:  "component1",
					Application:    "application1",
					ContainerImage: "image",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.newComp.ValidateCreate()

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestComponentUpdateValidatingWebhook(t *testing.T) {

	originalComponent := Component{
		Spec: ComponentSpec{
			ComponentName: "component",
			Application:   "application",
			Source: ComponentSource{
				ComponentSourceUnion: ComponentSourceUnion{
					GitSource: &GitSource{
						URL:     "http://link",
						Context: "context",
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		updateComp Component
		err        string
	}{
		{
			name: "component name cannot be changed",
			err:  "component name cannot be updated to",
			updateComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component1",
				},
			},
		},
		{
			name: "application name cannot be changed",
			err:  "application name cannot be updated to",
			updateComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component",
					Application:   "application1",
				},
			},
		},
		{
			name: "git src cannot be changed",
			err:  "git source cannot be updated to",
			updateComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component",
					Application:   "application",
					Source: ComponentSource{
						ComponentSourceUnion: ComponentSourceUnion{
							GitSource: &GitSource{
								URL:     "http://link1",
								Context: "context",
							},
						},
					},
				},
			},
		},
		{
			name: "container image can be changed",
			updateComp: Component{
				Spec: ComponentSpec{
					ComponentName:  "component",
					Application:    "application",
					ContainerImage: "image1",
				},
			},
		},
		{
			name: "not component",
			err:  "runtime object is not of type Component",
			updateComp: Component{
				Spec: ComponentSpec{
					ComponentName: "component1",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err == "" {
				originalComponent = Component{
					Spec: ComponentSpec{
						ComponentName:  "component",
						Application:    "application",
						ContainerImage: "image",
						Source: ComponentSource{
							ComponentSourceUnion: ComponentSourceUnion{
								GitSource: &GitSource{
									Context: "context",
								},
							},
						},
					},
				}
			}
			var err error
			if test.name == "not component" {
				originalApplication := Application{
					Spec: ApplicationSpec{
						DisplayName: "My App",
					},
				}
				err = test.updateComp.ValidateUpdate(&originalApplication)
			} else {
				err = test.updateComp.ValidateUpdate(&originalComponent)
			}

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestComponentDeleteValidatingWebhook(t *testing.T) {

	tests := []struct {
		name    string
		newComp Component
		err     string
	}{
		{
			name:    "ValidateDelete should return nil, it's unimplimented",
			err:     "",
			newComp: Component{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.newComp.ValidateDelete()

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

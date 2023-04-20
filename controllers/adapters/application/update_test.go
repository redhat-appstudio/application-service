package application

import (
	"testing"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
)

func TestUpdateApplicationDevfileModel(t *testing.T) {
	tests := []struct {
		name           string
		projects       []devfileAPIV1.Project
		attributes     attributes.Attributes
		containerImage string
		component      appstudiov1alpha1.Component
		wantErr        bool
	}{
		{
			name: "Project already present",
			projects: []devfileAPIV1.Project{
				{
					Name: "duplicate",
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "duplicate",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Project added successfully",
			projects: []devfileAPIV1.Project{
				{
					Name: "present",
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "new",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
				},
			},
		},
		{
			name: "Git source in Component is nil",
			projects: []devfileAPIV1.Project{
				{
					Name: "present",
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "new",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "Devfile Projects list is nil",
			projects: nil,
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "new",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Container image added successfully",
			attributes: attributes.Attributes{}.PutString("containerImage/otherComponent", "other-image"),
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "new",
					ContainerImage: "an-image",
				},
			},
		},
		{
			name:       "Container image already exists",
			attributes: attributes.Attributes{}.PutString("containerImage/new", "an-image"),
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "new",
					ContainerImage: "an-image",
				},
			},
			wantErr: true,
		},
		{
			name:       "Container image already exists, but invalid entry",
			attributes: attributes.Attributes{}.Put("containerImage/new", make(chan error), nil),
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "new",
					ContainerImage: "an-image",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileData := &v2.DevfileV2{
				Devfile: devfileAPIV1.Devfile{
					DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
							Attributes: tt.attributes,
							Projects:   tt.projects,
						},
					},
				},
			}
			err := updateApplicationDevfileModel(devfileData, tt.component)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				if tt.component.Spec.Source.GitSource != nil {
					projects, err := devfileData.GetProjects(common.DevfileOptions{})
					if err != nil {
						t.Errorf("got unexpected error: %v", err)
					}
					matched := false
					for _, project := range projects {
						projectGitSrc := project.ProjectSource.Git
						if project.Name == tt.component.Spec.ComponentName && projectGitSrc != nil && projectGitSrc.Remotes["origin"] == tt.component.Spec.Source.GitSource.URL {
							matched = true
						}
					}

					if !matched {
						t.Errorf("unable to find devfile with project: %s", tt.component.Spec.ComponentName)
					}

				} else {
					devfileAttr, err := devfileData.GetAttributes()
					if err != nil {
						t.Errorf("got unexpected error: %v", err)
					}
					if devfileAttr == nil {
						t.Errorf("devfile attributes should not be nil")
					}
					containerImage := devfileAttr.GetString("containerImage/new", &err)
					if err != nil {
						t.Errorf("got unexpected error: %v", err)
					}
					if containerImage != tt.component.Spec.ContainerImage {
						t.Errorf("unable to find component with container iamge: %s", tt.component.Spec.ContainerImage)
					}
				}
			}
		})
	}
}

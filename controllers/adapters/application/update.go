package application

import (
	"fmt"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
)

// updateApplicationDevfileModel updates the Application's devfile model to include
func updateApplicationDevfileModel(applicationDevfileData data.DevfileData, component appstudiov1alpha1.Component) error {

	if component.Spec.Source.GitSource != nil {
		newProject := devfileAPIV1.Project{
			Name: component.Spec.ComponentName,
			ProjectSource: devfileAPIV1.ProjectSource{
				Git: &devfileAPIV1.GitProjectSource{
					GitLikeProjectSource: devfileAPIV1.GitLikeProjectSource{
						Remotes: map[string]string{
							"origin": component.Spec.Source.GitSource.URL,
						},
					},
				},
			},
		}
		projects, err := applicationDevfileData.GetProjects(common.DevfileOptions{})
		if err != nil {
			return err
		}
		for _, project := range projects {
			if project.Name == newProject.Name {
				return fmt.Errorf("application already has a component with name %s", newProject.Name)
			}
		}
		err = applicationDevfileData.AddProjects([]devfileAPIV1.Project{newProject})
		if err != nil {
			return err
		}
	} else if component.Spec.ContainerImage != "" {
		var err error

		// Initialize the attributes
		devSpec := applicationDevfileData.GetDevfileWorkspaceSpec()

		// Add the image as a top level attribute
		devfileAttributes := devSpec.Attributes
		if devfileAttributes == nil {
			devfileAttributes = attributes.Attributes{}
			devSpec.Attributes = devfileAttributes
			applicationDevfileData.SetDevfileWorkspaceSpec(*devSpec)
		}
		imageAttrString := fmt.Sprintf("containerImage/%s", component.Spec.ComponentName)
		componentImage := devfileAttributes.GetString(imageAttrString, &err)
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return err
			}
		}
		if componentImage != "" {
			return fmt.Errorf("application already has a component with name %s", component.Name)
		}
		devSpec.Attributes = devfileAttributes.PutString(imageAttrString, component.Spec.ContainerImage)
		applicationDevfileData.SetDevfileWorkspaceSpec(*devSpec)

	} else {
		return fmt.Errorf("component source is nil")
	}

	return nil
}

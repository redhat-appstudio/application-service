package util

import (
	"net/url"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
)

func GetMappedGitOpsComponent(component appstudiov1alpha1.Component) gitopsgenv1alpha1.GeneratorOptions {
	customK8sLabels := map[string]string{
		"app.kubernetes.io/name":       component.Spec.ComponentName,
		"app.kubernetes.io/instance":   component.Name,
		"app.kubernetes.io/part-of":    component.Spec.Application,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
	gitopsMapComponent := gitopsgenv1alpha1.GeneratorOptions{
		Name:           component.ObjectMeta.Name,
		Namespace:      component.ObjectMeta.Namespace,
		Application:    component.Spec.Application,
		Secret:         component.Spec.Secret,
		Resources:      component.Spec.Resources,
		Replicas:       component.Spec.Replicas,
		TargetPort:     component.Spec.TargetPort,
		Route:          component.Spec.Route,
		BaseEnvVar:     component.Spec.Env,
		ContainerImage: component.Spec.ContainerImage,
		K8sLabels:      customK8sLabels,
	}
	if component.Spec.Source.ComponentSourceUnion.GitSource != nil {
		gitopsMapComponent.GitSource = &gitopsgenv1alpha1.GitSource{
			URL: component.Spec.Source.ComponentSourceUnion.GitSource.URL,
		}
	} else {
		gitopsMapComponent.GitSource = &gitopsgenv1alpha1.GitSource{}
	}
	return gitopsMapComponent
}

func GetRemoteURL(gitOpsURL string, gitToken string) (string, error) {
	parsedURL, err := url.Parse(gitOpsURL)
	if err != nil {
		return "", err
	}
	parsedURL.User = url.User(gitToken)
	remoteURL := parsedURL.String()
	return remoteURL, nil
}

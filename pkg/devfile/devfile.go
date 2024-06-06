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

package devfile

import (
	"strconv"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/generator"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

// GetIngressFromEndpoint gets an ingress resource from the devfile endpoint information
func GetIngressFromEndpoint(name, serviceName, port, path string, secure bool, annotations map[string]string, hostname string) (networkingv1.Ingress, error) {

	if path == "" {
		path = "/"
	}

	implementationSpecific := networkingv1.PathTypeImplementationSpecific

	portNumber, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return networkingv1.Ingress{}, err
	}

	ingress := networkingv1.Ingress{
		TypeMeta:   generator.GetTypeMeta("Ingress", "networking.k8s.io/v1"),
		ObjectMeta: generator.GetObjectMeta(name, "", nil, annotations),
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &implementationSpecific,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(portNumber),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return ingress, nil
}

// GetRouteFromEndpoint gets the route resource
func GetRouteFromEndpoint(name, serviceName, port, path string, secure bool, annotations map[string]string) routev1.Route {

	if path == "" {
		path = "/"
	}

	routeParams := generator.RouteParams{
		ObjectMeta: generator.GetObjectMeta(name, "", nil, nil),
		TypeMeta:   generator.GetTypeMeta("Route", "route.openshift.io/v1"),
		RouteSpecParams: generator.RouteSpecParams{
			ServiceName: serviceName,
			PortNumber:  intstr.FromString(port),
			Path:        path,
			Secure:      secure,
		},
	}

	return *generator.GetRoute(v1alpha2.Endpoint{Annotations: annotations}, routeParams)
}

// ConvertApplicationToDevfile takes in a given Application CR and converts it to
// a devfile object
func ConvertApplicationToDevfile(hasApp appstudiov1alpha1.Application) (data.DevfileData, error) {
	devfileVersion := string(data.APISchemaVersion220)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return nil, err
	}

	devfileData.SetSchemaVersion(devfileVersion)

	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name:        hasApp.Spec.DisplayName,
		Description: hasApp.Spec.Description,
	})

	return devfileData, nil
}

func ConvertImageComponentToDevfile(comp appstudiov1alpha1.Component) (data.DevfileData, error) {
	devfileVersion := string(data.APISchemaVersion220)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return nil, err
	}

	devfileData.SetSchemaVersion(devfileVersion)
	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name: comp.Spec.ComponentName,
	})

	deploymentTemplate := GenerateDeploymentTemplate(comp.Name, comp.Spec.Application, comp.Spec.ContainerImage)
	deploymentTemplateBytes, err := yaml.Marshal(deploymentTemplate)
	if err != nil {
		return nil, err
	}

	// Generate a stub container component for the devfile
	components := []v1alpha2.Component{
		{
			Name: "kubernetes-deploy",
			ComponentUnion: v1alpha2.ComponentUnion{
				Kubernetes: &v1alpha2.KubernetesComponent{
					K8sLikeComponent: v1alpha2.K8sLikeComponent{
						K8sLikeComponentLocation: v1alpha2.K8sLikeComponentLocation{
							Inlined: string(deploymentTemplateBytes),
						},
					},
				},
			},
		},
	}

	err = devfileData.AddComponents(components)
	if err != nil {
		return nil, err
	}

	return devfileData, nil
}

// CreateDevfileForDockerfileBuild creates a devfile with the Dockerfile uri and build context
func CreateDevfileForDockerfileBuild(dockerfileUri, buildContext, name, application string) (data.DevfileData, error) {
	devfileVersion := string(data.APISchemaVersion220)
	devfileData, err := data.NewDevfileData(devfileVersion)
	if err != nil {
		return nil, err
	}

	devfileData.SetSchemaVersion(devfileVersion)

	devfileData.SetMetadata(devfile.DevfileMetadata{
		Name:        "dockerfile-component",
		Description: "Basic Devfile for a Dockerfile Component",
	})

	deploymentTemplate := GenerateDeploymentTemplate(name, application, "")
	deploymentTemplateBytes, err := yaml.Marshal(deploymentTemplate)
	if err != nil {
		return nil, err
	}

	components := []v1alpha2.Component{
		{
			Name: "dockerfile-build",
			ComponentUnion: v1alpha2.ComponentUnion{
				Image: &v1alpha2.ImageComponent{
					Image: v1alpha2.Image{
						ImageUnion: v1alpha2.ImageUnion{
							Dockerfile: &v1alpha2.DockerfileImage{
								DockerfileSrc: v1alpha2.DockerfileSrc{
									Uri: dockerfileUri,
								},
								Dockerfile: v1alpha2.Dockerfile{
									BuildContext: buildContext,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "kubernetes-deploy",
			ComponentUnion: v1alpha2.ComponentUnion{
				Kubernetes: &v1alpha2.KubernetesComponent{
					K8sLikeComponent: v1alpha2.K8sLikeComponent{
						K8sLikeComponentLocation: v1alpha2.K8sLikeComponentLocation{
							Inlined: string(deploymentTemplateBytes),
						},
					},
				},
			},
		},
	}
	err = devfileData.AddComponents(components)
	if err != nil {
		return nil, err
	}

	commands := []v1alpha2.Command{
		{
			Id: "build-image",
			CommandUnion: v1alpha2.CommandUnion{
				Apply: &v1alpha2.ApplyCommand{
					Component: "dockerfile-build",
				},
			},
		},
	}
	err = devfileData.AddCommands(commands)
	if err != nil {
		return nil, err
	}

	return devfileData, nil
}

// GenerateDeploymentTemplate generates a deployment template with the information passed
func GenerateDeploymentTemplate(name, application, image string) appsv1.Deployment {

	k8sLabels := generateK8sLabels(name, application)
	matchLabels := getMatchLabel(name)

	containerImage := "image"
	if image != "" {
		containerImage = image
	}

	deployment := appsv1.Deployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:   name,
			Labels: k8sLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: matchLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "container-image",
							Image:           containerImage,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
		},
	}

	return deployment

}

func generateK8sLabels(name, application string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/part-of":    application,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
}

// GetMatchLabel returns the label selector that will be used to tie deployments, services, and pods together
// For cleanliness, using just one unique label from the generateK8sLabels function
func getMatchLabel(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/instance": name,
	}
}

// FindAndDownloadDevfile downloads devfile from the various possible devfile locations in dir and returns the contents and its context
func FindAndDownloadDevfile(dir, token string) ([]byte, string, error) {
	var devfileBytes []byte
	var err error

	for _, path := range cdqanalysis.ValidDevfileLocations {
		devfilePath := dir + "/" + path
		devfileBytes, err = DownloadFile(devfilePath, token)
		if err == nil {
			// if we get a 200, return
			return devfileBytes, path, err
		}
	}

	return nil, "", &cdqanalysis.NoDevfileFound{Location: dir}
}

// FindAndDownloadDockerfile downloads Dockerfile from the various possible Dockerfile, or Containerfile locations in dir and returns the contents and its context
func FindAndDownloadDockerfile(dir, token string) ([]byte, string, error) {
	var dockerfileBytes []byte
	var err error
	// Containerfile is an alternate name for Dockerfile

	for _, path := range cdqanalysis.ValidDockerfileLocations {
		dockerfilePath := dir + "/" + path
		dockerfileBytes, err = DownloadFile(dockerfilePath, token)
		if err == nil {
			// if we get a 200, return
			return dockerfileBytes, path, err
		}
	}

	return nil, "", &cdqanalysis.NoDockerfileFound{Location: dir}
}

// DownloadFile downloads the specified file
func DownloadFile(file, token string) ([]byte, error) {
	return cdqanalysis.CurlEndpoint(file, token)
}

// UpdateLocalDockerfileURItoAbsolute takes in a Devfile, and a DockefileURL, and returns back a Devfile with any local URIs to the Dockerfile updates to be absolute
func UpdateLocalDockerfileURItoAbsolute(devfile data.DevfileData, dockerfileURL string) (data.DevfileData, error) {
	devfileComponents, err := devfile.GetComponents(common.DevfileOptions{ComponentOptions: common.ComponentOptions{
		ComponentType: v1alpha2.ImageComponentType,
	}})
	if err != nil {
		return nil, err
	}

	for _, comp := range devfileComponents {
		if comp.Image != nil && comp.Image.Dockerfile != nil {
			comp.Image.Dockerfile.Uri = dockerfileURL

			// Update the component in the devfile
			err = devfile.UpdateComponent(comp)
			if err != nil {
				return nil, err
			}
		}
	}

	return devfile, err
}

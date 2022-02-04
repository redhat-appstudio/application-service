//
// Copyright 2021-2022 Red Hat, Inc.
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

package gitops

import (
	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/resources"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	yaml "github.com/redhat-appstudio/application-service/gitops/yaml"
)

const (
	kustomizeFileName  = "kustomization.yaml"
	deploymentFileName = "deployment.yaml"
	serviceFileName    = "service.yaml"
	routeFileName      = "route.yaml"
)

// Generate takes in a given Component CR and
// spits out a deployment, service, and route file to disk
func Generate(fs afero.Fs, outputFolder string, component appstudiov1alpha1.Component) error {
	deployment := generateDeployment(component)

	k := resources.Kustomization{}
	k.AddResources(deploymentFileName)
	resources := map[string]interface{}{
		deploymentFileName: deployment,
	}

	// If a targetPort was specified, also generate a service and route
	if component.Spec.TargetPort != 0 {
		service := generateService(component)
		route := generateRoute(component)
		k.AddResources(deploymentFileName, serviceFileName, routeFileName)
		resources[serviceFileName] = service
		resources[routeFileName] = route
	}
	resources[kustomizeFileName] = k

	_, err := yaml.WriteResources(fs, outputFolder, resources)
	return err
}

func generateDeployment(component appstudiov1alpha1.Component) *appsv1.Deployment {
	replicas := getReplicas(component)

	labels := map[string]string{
		"component": component.Name,
	}
	deployment := appsv1.Deployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &v1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "container-image",
							Image:           component.Spec.Build.ContainerImage,
							ImagePullPolicy: corev1.PullAlways,
							Env:             component.Spec.Env,
							Resources:       component.Spec.Resources,
						},
					},
				},
			},
		},
	}

	// Set fields that may have been optionally configured by the component CR
	if component.Spec.TargetPort != 0 {
		deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{
				ContainerPort: int32(component.Spec.TargetPort),
			},
		}
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(component.Spec.TargetPort),
				},
			},
		}
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Port: intstr.FromInt(component.Spec.TargetPort),
					Path: "/",
				},
			},
		}
	}

	return &deployment
}

func generateService(component appstudiov1alpha1.Component) *corev1.Service {
	labels := map[string]string{
		"component": component.Name,
	}
	service := corev1.Service{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       int32(component.Spec.TargetPort),
					TargetPort: intstr.FromInt(component.Spec.TargetPort),
				},
			},
		},
	}

	return &service
}

func generateRoute(component appstudiov1alpha1.Component) *routev1.Route {
	labels := map[string]string{
		"component": component.Name,
	}
	weight := int32(100)
	route := routev1.Route{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(component.Spec.TargetPort),
			},
			TLS: &routev1.TLSConfig{
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routev1.TLSTerminationEdge,
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   component.Name,
				Weight: &weight,
			},
		},
	}

	// If the route field is set in the spec, set it to be the host for the route
	if component.Spec.Route != "" {
		route.Spec.Host = component.Spec.Route
	}

	return &route
}

// getReplicas returns the number of replicas to be created for the component
// If the field is not set, it returns a default value of 1
// ToDo: Handle as part of a defaulting webhook
func getReplicas(component appstudiov1alpha1.Component) int32 {
	if component.Spec.Replicas > 0 {
		return int32(component.Spec.Replicas)
	}
	return 1
}

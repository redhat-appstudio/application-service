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
	"fmt"
	"regexp"
	"strconv"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	devfilePkg "github.com/devfile/library/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/generator"
	parser "github.com/devfile/library/v2/pkg/devfile/parser"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	"golang.org/x/exp/maps"

	"github.com/redhat-appstudio/application-service/pkg/util"

	"github.com/go-logr/logr"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// DevfileSrc specifies the src of the Devfile
type DevfileSrc struct {
	Data string
	URL  string
	Path string
}

func GetResourceFromDevfile(log logr.Logger, devfileData data.DevfileData, deployAssociatedComponents map[string]string, compName, appName, image, hostname string) (parser.KubernetesResources, error) {
	kubernetesComponentFilter := common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.KubernetesComponentType,
		},
	}
	kubernetesComponents, err := devfileData.GetComponents(kubernetesComponentFilter)
	if err != nil {
		return parser.KubernetesResources{}, err
	}

	var appendedResources parser.KubernetesResources
	k8sLabels := generateK8sLabels(compName, appName)
	matchLabels := getMatchLabel(compName)

	if len(kubernetesComponents) == 0 {
		return parser.KubernetesResources{}, fmt.Errorf("the devfile has no kubernetes components defined, missing outerloop definition")
	} else if len(kubernetesComponents) == 1 && len(deployAssociatedComponents) == 0 {
		// only one kubernetes components defined, but no deploy cmd associated
		deployAssociatedComponents[kubernetesComponents[0].Name] = "place-holder"
	}
	for _, component := range kubernetesComponents {
		// get kubecomponent referenced by default deploy command
		if _, ok := deployAssociatedComponents[component.Name]; ok && component.Kubernetes != nil {
			if component.Kubernetes.Inlined != "" {
				log.Info(fmt.Sprintf("reading the kubernetes inline from component %s", component.Name))
				src := parser.YamlSrc{
					Data: []byte(component.Kubernetes.Inlined),
				}
				values, err := parser.ReadKubernetesYaml(src, nil)
				if err != nil {
					return parser.KubernetesResources{}, err
				}

				resources, err := parser.ParseKubernetesYaml(values)
				if err != nil {
					return parser.KubernetesResources{}, err
				}

				var endpointRoutes []routev1.Route
				var endpointIngresses []networkingv1.Ingress
				for _, endpoint := range component.Kubernetes.Endpoints {
					if endpoint.Exposure != v1alpha2.NoneEndpointExposure && endpoint.Exposure != v1alpha2.InternalEndpointExposure {
						var isSecure bool
						if endpoint.Secure != nil {
							isSecure = *endpoint.Secure
						}

						ingressEndpoint, err := GetIngressFromEndpoint(endpoint.Name, compName, fmt.Sprintf("%d", endpoint.TargetPort), endpoint.Path, isSecure, endpoint.Annotations, hostname)
						if err != nil {
							return parser.KubernetesResources{}, err
						}
						endpointIngresses = append(endpointIngresses, ingressEndpoint)

						endpointRoutes = append(endpointRoutes, GetRouteFromEndpoint(endpoint.Name, compName, fmt.Sprintf("%d", endpoint.TargetPort), endpoint.Path, isSecure, endpoint.Annotations))
					}
				}
				// attempt to always merge the devfile endpoints to the list first as it has priority
				resources.Routes = append(endpointRoutes, resources.Routes...)
				resources.Ingresses = append(endpointIngresses, resources.Ingresses...)

				// update for port
				currentPort := int(component.Attributes.GetNumber(ContainerImagePortKey, &err))
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return parser.KubernetesResources{}, err
					}
				}

				// update for ENV
				currentENV := []corev1.EnvVar{}
				err = component.Attributes.GetInto(ContainerENVKey, &currentENV)
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return parser.KubernetesResources{}, err
					}
				}

				if len(resources.Deployments) > 0 {
					// update for replica
					currentReplica := int32(component.Attributes.GetNumber(ReplicaKey, &err))
					if err != nil {
						if _, ok := err.(*attributes.KeyNotFoundError); !ok {
							return parser.KubernetesResources{}, err
						}
					}

					// Set the RevisionHistoryLimit for all Deployments to 0, if it's unset
					// If set, leave it alone
					for i := range resources.Deployments {
						if resources.Deployments[i].Spec.RevisionHistoryLimit == nil {
							resources.Deployments[i].Spec.RevisionHistoryLimit = &util.RevisionHistoryLimit
						}
					}

					// replace the deployment metadata.name to use the component name
					resources.Deployments[0].ObjectMeta.Name = compName

					// generate and append the deployment labels with the hc & ha information
					if resources.Deployments[0].ObjectMeta.Labels != nil {
						maps.Copy(resources.Deployments[0].ObjectMeta.Labels, k8sLabels)
					} else {
						resources.Deployments[0].ObjectMeta.Labels = k8sLabels
					}
					if resources.Deployments[0].Spec.Selector != nil {
						if resources.Deployments[0].Spec.Selector.MatchLabels != nil {
							maps.Copy(resources.Deployments[0].Spec.Selector.MatchLabels, matchLabels)
						} else {
							resources.Deployments[0].Spec.Selector.MatchLabels = matchLabels
						}
					} else {
						resources.Deployments[0].Spec.Selector = &v1.LabelSelector{
							MatchLabels: matchLabels,
						}
					}
					if resources.Deployments[0].Spec.Template.ObjectMeta.Labels != nil {
						maps.Copy(resources.Deployments[0].Spec.Template.ObjectMeta.Labels, matchLabels)
					} else {
						resources.Deployments[0].Spec.Template.ObjectMeta.Labels = matchLabels
					}

					if currentReplica > 0 {
						resources.Deployments[0].Spec.Replicas = &currentReplica
					}

					if len(resources.Deployments[0].Spec.Template.Spec.Containers) > 0 {
						if image != "" {
							resources.Deployments[0].Spec.Template.Spec.Containers[0].Image = image
						}

						if currentPort > 0 {
							containerPort := corev1.ContainerPort{
								ContainerPort: int32(currentPort),
							}

							isPresent := false
							for _, port := range resources.Deployments[0].Spec.Template.Spec.Containers[0].Ports {
								if port.ContainerPort == containerPort.ContainerPort {
									isPresent = true
									break
								}
							}

							if !isPresent {
								resources.Deployments[0].Spec.Template.Spec.Containers[0].Ports = append(resources.Deployments[0].Spec.Template.Spec.Containers[0].Ports, containerPort)
							}

							if resources.Deployments[0].Spec.Template.Spec.Containers[0].ReadinessProbe != nil && resources.Deployments[0].Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.TCPSocket != nil {
								resources.Deployments[0].Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.TCPSocket.Port.IntVal = int32(currentPort)
							}

							if resources.Deployments[0].Spec.Template.Spec.Containers[0].LivenessProbe != nil && resources.Deployments[0].Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.HTTPGet != nil {
								resources.Deployments[0].Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.HTTPGet.Port.IntVal = int32(currentPort)
							}
						}

						for _, devfileEnv := range currentENV {
							isPresent := false
							for i, containerEnv := range resources.Deployments[0].Spec.Template.Spec.Containers[0].Env {
								if containerEnv.Name == devfileEnv.Name {
									isPresent = true
									resources.Deployments[0].Spec.Template.Spec.Containers[0].Env[i].Value = devfileEnv.Value
								}
							}

							if !isPresent {
								resources.Deployments[0].Spec.Template.Spec.Containers[0].Env = append(resources.Deployments[0].Spec.Template.Spec.Containers[0].Env, devfileEnv)
							}
						}

						// Update for limits
						cpuLimit := component.Attributes.GetString(CpuLimitKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						memoryLimit := component.Attributes.GetString(MemoryLimitKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						storageLimit := component.Attributes.GetString(StorageLimitKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						containerLimits := resources.Deployments[0].Spec.Template.Spec.Containers[0].Resources.Limits
						if len(containerLimits) == 0 {
							containerLimits = make(corev1.ResourceList)
						}

						if cpuLimit != "" && cpuLimit != "0" {
							cpuLimitQuantity, err := resource.ParseQuantity(cpuLimit)
							if err != nil {
								return parser.KubernetesResources{}, err
							}
							containerLimits[corev1.ResourceCPU] = cpuLimitQuantity
						}

						if memoryLimit != "" && memoryLimit != "0" {
							memoryLimitQuantity, err := resource.ParseQuantity(memoryLimit)
							if err != nil {
								return parser.KubernetesResources{}, err
							}
							containerLimits[corev1.ResourceMemory] = memoryLimitQuantity
						}

						if storageLimit != "" && storageLimit != "0" {
							storageLimitQuantity, err := resource.ParseQuantity(storageLimit)
							if err != nil {
								return parser.KubernetesResources{}, err
							}
							containerLimits[corev1.ResourceStorage] = storageLimitQuantity
						}

						resources.Deployments[0].Spec.Template.Spec.Containers[0].Resources.Limits = containerLimits

						// Update for requests
						cpuRequest := component.Attributes.GetString(CpuRequestKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						memoryRequest := component.Attributes.GetString(MemoryRequestKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						storageRequest := component.Attributes.GetString(StorageRequestKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						containerRequests := resources.Deployments[0].Spec.Template.Spec.Containers[0].Resources.Requests
						if len(containerRequests) == 0 {
							containerRequests = make(corev1.ResourceList)
						}

						if cpuRequest != "" && cpuRequest != "0" {
							cpuRequestQuantity, err := resource.ParseQuantity(cpuRequest)
							if err != nil {
								return parser.KubernetesResources{}, err
							}
							containerRequests[corev1.ResourceCPU] = cpuRequestQuantity
						}

						if memoryRequest != "" && memoryRequest != "0" {
							memoryRequestQuantity, err := resource.ParseQuantity(memoryRequest)
							if err != nil {
								return parser.KubernetesResources{}, err
							}
							containerRequests[corev1.ResourceMemory] = memoryRequestQuantity
						}

						if storageRequest != "" && storageRequest != "0" {
							storageRequestQuantity, err := resource.ParseQuantity(storageRequest)
							if err != nil {
								return parser.KubernetesResources{}, err
							}
							containerRequests[corev1.ResourceStorage] = storageRequestQuantity
						}

						resources.Deployments[0].Spec.Template.Spec.Containers[0].Resources.Requests = containerRequests
					}
				}

				if len(resources.Services) > 0 {
					// replace the service metadata.name to use the component name
					resources.Services[0].ObjectMeta.Name = compName

					// generate and append the service labels with the hc & ha information
					if resources.Services[0].ObjectMeta.Labels != nil {
						maps.Copy(resources.Services[0].ObjectMeta.Labels, k8sLabels)
					} else {
						resources.Services[0].ObjectMeta.Labels = k8sLabels
					}
					if resources.Services[0].Spec.Selector != nil {
						maps.Copy(resources.Services[0].Spec.Selector, matchLabels)
					} else {
						resources.Services[0].Spec.Selector = matchLabels
					}

					if currentPort > 0 {
						servicePort := corev1.ServicePort{
							Port:       int32(currentPort),
							TargetPort: intstr.FromInt(currentPort),
						}

						isPresent := false
						for _, port := range resources.Services[0].Spec.Ports {
							if port.Port == servicePort.Port {
								isPresent = true
								break
							}
						}

						if !isPresent {
							resources.Services[0].Spec.Ports = append(resources.Services[0].Spec.Ports, servicePort)
						}
					}
				}
				if len(resources.Routes) > 0 {
					// replace the route metadata.name to use the component name
					// Trim the route name if needed
					routeName := compName
					if len(routeName) >= 30 {
						routeName = routeName[0:25] + util.GetRandomString(4, true)
					}

					resources.Routes[0].ObjectMeta.Name = routeName

					// generate and append the route labels with the hc & ha information
					if resources.Routes[0].ObjectMeta.Labels != nil {
						maps.Copy(resources.Routes[0].ObjectMeta.Labels, k8sLabels)
					} else {
						resources.Routes[0].ObjectMeta.Labels = k8sLabels
					}

					if currentPort > 0 {
						if resources.Routes[0].Spec.Port == nil {
							resources.Routes[0].Spec.Port = &routev1.RoutePort{}
						}
						resources.Routes[0].Spec.Port.TargetPort = intstr.FromInt(currentPort)
						// Update for route
						route := component.Attributes.GetString(RouteKey, &err)
						if err != nil {
							if _, ok := err.(*attributes.KeyNotFoundError); !ok {
								return parser.KubernetesResources{}, err
							}
						}

						if route != "" {
							resources.Routes[0].Spec.Host = route
						}
					}
				}
				if len(resources.Ingresses) > 0 {
					// replace the ingress metadata.name to use the component name
					ingressName := compName

					resources.Ingresses[0].ObjectMeta.Name = ingressName

					// generate and append the ingress labels with the hc & ha information
					if resources.Ingresses[0].ObjectMeta.Labels != nil {
						maps.Copy(resources.Ingresses[0].ObjectMeta.Labels, k8sLabels)
					} else {
						resources.Ingresses[0].ObjectMeta.Labels = k8sLabels
					}
					if currentPort > 0 {
						if len(resources.Ingresses[0].Spec.Rules) > 0 {
							if resources.Ingresses[0].Spec.Rules[0].HTTP != nil && len(resources.Ingresses[0].Spec.Rules[0].HTTP.Paths) > 0 {
								if resources.Ingresses[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service != nil {
									resources.Ingresses[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number = int32(currentPort)
								}
							}
						}
					}
				}

				appendedResources.Deployments = append(appendedResources.Deployments, resources.Deployments...)
				appendedResources.Services = append(appendedResources.Services, resources.Services...)
				appendedResources.Routes = append(appendedResources.Routes, resources.Routes...)
				appendedResources.Ingresses = append(appendedResources.Ingresses, resources.Ingresses...)
				appendedResources.Others = append(appendedResources.Others, resources.Others...)
			} else {
				log.Info(fmt.Sprintf("Kubernetes Component %s did not have an inline content, gitOps resources may be auto generated", component.Name))
			}
		}
	}

	return appendedResources, err
}

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

// ParseDevfile calls the devfile library's parse and returns the devfile data.
// Provide either a Data src or the URL src
func ParseDevfile(src DevfileSrc) (data.DevfileData, error) {

	httpTimeout := 10
	convert := true
	parserArgs := parser.ParserArgs{
		HTTPTimeout:                   &httpTimeout,
		ConvertKubernetesContentInUri: &convert,
	}

	if src.Data != "" {
		parserArgs.Data = []byte(src.Data)
	} else if src.URL != "" {
		parserArgs.URL = src.URL
	} else if src.Path != "" {
		parserArgs.Path = src.Path
	} else {
		return nil, fmt.Errorf("cannot parse devfile without a src")
	}
	devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parserArgs)
	return devfileObj.Data, err
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

// GetIngressHostName gets the ingress host name from the component name, namepsace and ingress domain
func GetIngressHostName(componentName, namespace, ingressDomain string) (string, error) {

	regexString := `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`
	ingressHostRegex := regexp.MustCompile(regexString)

	host := fmt.Sprintf("%s-%s.%s", componentName, namespace, ingressDomain)

	if !ingressHostRegex.MatchString(host) {
		return "", fmt.Errorf("hostname %s should match regex %s", host, regexString)
	}

	return host, nil
}

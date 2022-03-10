/*
Copyright 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gitops

import (
	"encoding/json"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/resources"
	yaml "github.com/redhat-appstudio/application-service/gitops/yaml"
	"github.com/spf13/afero"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	triggersapi "github.com/tektoncd/triggers/pkg/apis/triggers/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	buildCommonStoragePVCFileName = "common-storage-pvc.yaml"
	buildTriggerTemplateFileName  = "trigger-template.yaml"
	buildEventListenerFileName    = "event-listener.yaml"
	buildWebhookRouteFileName     = "build-webhook-route.yaml"
)

func GenerateBuild(fs afero.Fs, outputFolder string, component appstudiov1alpha1.Component) error {
	commonStoragePVC := GenerateCommonStorage(component, "appstudio")

	triggerTemplate, err := GenerateTriggerTemplate(component)
	if err != nil {
		return err
	}
	eventListener := GenerateEventListener(component, *triggerTemplate)
	webhookRoute := GenerateBuildWebhookRoute(component)

	buildResources := map[string]interface{}{
		buildCommonStoragePVCFileName: commonStoragePVC,
		buildTriggerTemplateFileName:  triggerTemplate,
		buildEventListenerFileName:    eventListener,
		buildWebhookRouteFileName:     webhookRoute,
	}

	kustomize := resources.Kustomization{}
	for fileName := range buildResources {
		kustomize.AddResources(fileName)
	}

	buildResources[kustomizeFileName] = kustomize

	if _, err = yaml.WriteResources(fs, outputFolder, buildResources); err != nil {
		return err
	}

	return nil
}

// DetermineBuildExecution returns the pipelineRun spec that would be used
// in webhooks-triggered pipelineRuns as well as user-triggered PipelineRuns
func DetermineBuildExecution(component appstudiov1alpha1.Component, params []tektonapi.Param, workspaceSubPath string) tektonapi.PipelineRunSpec {

	pipelineRunSpec := tektonapi.PipelineRunSpec{
		Params: params,
		PipelineRef: &tektonapi.PipelineRef{

			// This can't be hardcoded to devfile-build.
			// The logic should determine if it is a
			// nodejs build, java build, dockerfile build or a devfile build.
			Name:   determineBuildDefinition(component),
			Bundle: determineBuildCatalog(component.Namespace),
		},

		Workspaces: []tektonapi.WorkspaceBinding{
			{
				Name: "workspace",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "appstudio",
				},
				SubPath: component.Name + "/" + workspaceSubPath,
			},
			{
				Name: "registry-auth",
				Secret: &corev1.SecretVolumeSource{
					SecretName: "redhat-appstudio-registry-pull-secret",
				},
			},
		},
	}
	return pipelineRunSpec
}

func determineBuildDefinition(component appstudiov1alpha1.Component) string {
	return "devfile-build"
}

func determineBuildCatalog(namespace string) string {
	// TODO: If there's a namespace/workspace specific catalog, we got
	// to respect that.
	return "quay.io/redhat-appstudio/build-templates-bundle@sha256:2205a29208fa686b47f841819f7abedb64adb93935493693892d0e18bbdbb77e"
}

func normalizeOutputImageURL(outputImage string) string {

	// if provided image format was
	//
	// quay.io/foo/bar:mytag , then
	// push to quay.io/foo/bar:mytag-git-revision
	//
	// else,
	// push to quay.io/foo/bar:git-revision

	if strings.Contains(outputImage, ":") {
		outputImage = outputImage + "-" + "$(tt.params.git-revision)"
	} else {
		outputImage = outputImage + ":" + "$(tt.params.git-revision)"
	}
	return outputImage
}

// GetParamsForComponentInitialBuild would return the 'input' parameters for the initial PipelineRun
// that would build an image from source right after a Component is imported.
func GetParamsForComponentInitialBuild(component appstudiov1alpha1.Component) []tektonapi.Param {
	sourceCode := component.Spec.Source.GitSource.URL
	outputImage := component.Spec.Build.ContainerImage

	params := []tektonapi.Param{
		{
			Name: "git-url",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: sourceCode,
			},
		},
		{
			Name: "output-image",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: outputImage,
			},
		},
	}

	return params
}

// getParamsForComponentWebhookBuilds returns the 'input' paramters for the webhook-triggered
// PipelineRuns. The key difference between webhook triggered PipelineRuns and user-triggered
// PipelineRuns would be that you'd have the git revision appended to the output image tag
func getParamsForComponentWebhookBuilds(component appstudiov1alpha1.Component) []tektonapi.Param {
	sourceCode := component.Spec.Source.GitSource.URL
	outputImage := normalizeOutputImageURL(component.Spec.Build.ContainerImage)

	params := []tektonapi.Param{
		{
			Name: "git-url",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: sourceCode,
			},
		},
		{
			Name: "output-image",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: outputImage,
			},
		},
	}

	return params
}

func GetBuildCommonLabelsForComponent(component *appstudiov1alpha1.Component) map[string]string {
	labels := map[string]string{
		"build.appstudio.openshift.io/build":       "true",
		"build.appstudio.openshift.io/type":        "build",
		"build.appstudio.openshift.io/version":     "0.1",
		"build.appstudio.openshift.io/component":   component.Name,
		"build.appstudio.openshift.io/application": component.Spec.Application,
	}
	return labels
}

// GenerateCommonStorage returns the PVC that would be created per namespace for
// user-triggered and webhook-triggered Tekton workspaces.
func GenerateCommonStorage(component appstudiov1alpha1.Component, name string) corev1.PersistentVolumeClaim {
	fsMode := corev1.PersistentVolumeFilesystem

	workspaceStorage := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   component.Namespace,
			Annotations: GetBuildCommonLabelsForComponent(&component),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("10Mi"),
				},
			},
			VolumeMode: &fsMode,
		},
	}
	return *workspaceStorage
}

// GenerateBuildWebhookRoute returns the Route resource that would enable
// ingress traffic into the webhook endpoint ( aka EventListener)
// TODO: This needs to be secure.
func GenerateBuildWebhookRoute(component appstudiov1alpha1.Component) routev1.Route {
	var port int32 = 8080
	webhook := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: "route.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "el" + component.Name,
			Namespace:   component.Namespace,
			Annotations: GetBuildCommonLabelsForComponent(&component),
		},
		Spec: routev1.RouteSpec{
			Path: "/",
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "el-" + component.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{IntVal: port},
			},
		},
	}
	return *webhook
}

// GenerateTriggerTemplate generates the TriggerTemplate resources
// which defines how a webhook-based trigger event would be handled -
// In this case, a PipelineRun to build an image would be created.
func GenerateTriggerTemplate(component appstudiov1alpha1.Component) (*triggersapi.TriggerTemplate, error) {
	webhookBasedBuildTemplate := DetermineBuildExecution(component, getParamsForComponentWebhookBuilds(component), "$(tt.params.git-revision)")
	resoureTemplatePipelineRun := tektonapi.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: component.Name + "-",
			Namespace:    component.Namespace,
			Annotations:  GetBuildCommonLabelsForComponent(&component),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineRun",
			APIVersion: "tekton.dev/v1beta1",
		},
		Spec: webhookBasedBuildTemplate,
	}
	resourceTemplatePipelineRunBytes, err := json.Marshal(resoureTemplatePipelineRun)
	if err != nil {
		return nil, err
	}
	triggerTemplate := triggersapi.TriggerTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TriggerTemplate",
			APIVersion: "triggers.tekton.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
		},
		Spec: triggersapi.TriggerTemplateSpec{
			Params: []triggersapi.ParamSpec{
				{
					Name: "git-revision",
				},
			},
			ResourceTemplates: []triggersapi.TriggerResourceTemplate{
				{
					RawExtension: runtime.RawExtension{Raw: resourceTemplatePipelineRunBytes},
				},
			},
		},
	}
	return &triggerTemplate, err
}

// The GenerateEventListener is responsible for defining how to "parse" the incoming event ( "github-push ")
// and create the resultant PipelineRun ( defined as a TriggerTemplate ).
// The reconciler for EventListeners create a Service, which when exposed enables
// ingress traffic from Github events.
func GenerateEventListener(component appstudiov1alpha1.Component, triggerTemplate triggersapi.TriggerTemplate) triggersapi.EventListener {
	eventListener := triggersapi.EventListener{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EventListener",
			APIVersion: "triggers.tekton.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: GetBuildCommonLabelsForComponent(&component),
		},
		Spec: triggersapi.EventListenerSpec{
			ServiceAccountName: "pipeline",
			Triggers: []triggersapi.EventListenerTrigger{
				{
					Bindings: []*triggersapi.TriggerSpecBinding{
						{
							Ref:  "github-push",
							Kind: triggersapi.ClusterTriggerBindingKind,
						},
					},
					Template: &triggersapi.TriggerSpecTemplate{
						Ref: &triggerTemplate.Name,
					},
				},
			},
		},
	}
	return eventListener
}

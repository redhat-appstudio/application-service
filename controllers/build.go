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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	triggersapi "github.com/tektoncd/triggers/pkg/apis/triggers/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func determineBuildDefinition(component appstudiov1alpha1.Component) string {
	return "devfile-build"
}

func determineBuildCatalog(namespace string) string {
	// TODO: If there's a namespace/workspace specific catalog, we got
	// to respect that.
	return "quay.io/redhat-appstudio/build-templates-bundle:v0.1.2"
}

// determineBuildExecution returns the pipelineRun spec
func determineBuildExecution(component appstudiov1alpha1.Component, params []tektonapi.Param, workspaceSubPath string) tektonapi.PipelineRunSpec {

	pipelineRunSpec := tektonapi.PipelineRunSpec{
		Params: params, //paramsForWebhookBasedBuilds(component),
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
				SubPath: component.Name + "/" + workspaceSubPath, //$(tt.params.git-revision)/",
			},
			{
				Name: "registry-auth",
				Secret: &corev1.SecretVolumeSource{
					SecretName: "redhat-appstudio-registry",
				},
			},
		},
	}
	return pipelineRunSpec
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

func paramsForInitialBuild(component appstudiov1alpha1.Component) []tektonapi.Param {
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

func paramsForWebhookBasedBuilds(component appstudiov1alpha1.Component) []tektonapi.Param {
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

func commonAnnotations() map[string]string {
	annotations := map[string]string{
		"build.appstudio.openshift.io/build":   "true",
		"build.appstudio.openshift.io/type":    "build",
		"build.appstudio.openshift.io/version": "0.1",
	}
	return annotations
}

func commonStorage(name string, namespace string) corev1.PersistentVolumeClaim {
	fsMode := corev1.PersistentVolumeFilesystem

	workspaceStorage := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: commonAnnotations(),
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

func (r *ComponentReconciler) setupWebhookTriggeredImageBuilds(ctx context.Context, log logr.Logger, component appstudiov1alpha1.Component) (string, error) {

	workspaceStorage := commonStorage("appstudio", component.Namespace)
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: workspaceStorage.Name, Namespace: workspaceStorage.Namespace}, pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(ctx, &workspaceStorage)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to create common storage %v", workspaceStorage))
				return "", err
			}
		} else {
			log.Error(err, fmt.Sprintf("Unable to get common storage %v", workspaceStorage))
			return "", err
		}
	}

	webhookBasedBuildTemplate := determineBuildExecution(component, paramsForWebhookBasedBuilds(component), "$(tt.params.git-revision)")

	resoureTemplatePipelineRun := tektonapi.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: component.Name + "-",
			Namespace:    component.Namespace,
			Annotations:  commonAnnotations(),
		},
		TypeMeta: v1.TypeMeta{
			Kind:       "PipelineRun",
			APIVersion: tektonapi.SchemeGroupVersion.Group + "/" + tektonapi.SchemeGroupVersion.Version,
		},
		Spec: webhookBasedBuildTemplate,
	}

	resourceTemplatePipelineRunBytes, err := json.Marshal(resoureTemplatePipelineRun)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to convert to bytes %v", resoureTemplatePipelineRun))

	}

	triggerTemplate := triggersapi.TriggerTemplate{}
	triggerTemplate.Name = component.Name
	triggerTemplate.Namespace = component.Namespace
	triggerTemplate.Spec = triggersapi.TriggerTemplateSpec{
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
	}

	err = controllerutil.SetOwnerReference(&component, &triggerTemplate, r.Scheme)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to set owner reference for %v", triggerTemplate))
	}
	err = r.Get(ctx, types.NamespacedName{Name: triggerTemplate.Name, Namespace: triggerTemplate.Namespace}, &triggersapi.TriggerTemplate{})
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(ctx, &triggerTemplate)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to create triggerTemplate %v", triggerTemplate))
				return "", err
			}
		} else {
			log.Error(err, fmt.Sprintf("Unable to get triggerTemplate %v", triggerTemplate))
			return "", err
		}
	}

	eventListener := triggersapi.EventListener{
		ObjectMeta: v1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: commonAnnotations(),
		},
		Spec: triggersapi.EventListenerSpec{
			// If left empty, the "default" service account would be used.
			// Should leave his empty?
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

	err = controllerutil.SetOwnerReference(&component, &eventListener, r.Scheme)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to set owner reference for %v", eventListener))
	}
	err = r.Get(ctx, types.NamespacedName{Name: eventListener.Name, Namespace: eventListener.Namespace}, &triggersapi.EventListener{})
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(ctx, &eventListener)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to create eventListener %v", eventListener))
				return "", err
			}
		} else {
			log.Error(err, fmt.Sprintf("Unable to get eventListener %v", eventListener))
			return "", err
		}
	}

	initialBuildSpec := determineBuildExecution(component, paramsForInitialBuild(component), "initialbuildpath")

	initialBuild := tektonapi.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: commonAnnotations(),
		},
		Spec: initialBuildSpec,
	}

	err = r.Get(ctx, types.NamespacedName{Name: initialBuild.Name, Namespace: initialBuild.Namespace}, &tektonapi.PipelineRun{})
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(ctx, &initialBuild)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to create initial build %v", initialBuild))
				return "", err
			}

		} else {
			log.Error(err, fmt.Sprintf("Unable to get build %v", eventListener))
			return "", err
		}
	}

	var port int32 = 8080
	webhook := &routev1.Route{
		ObjectMeta: v1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: commonAnnotations(),
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

	err = r.Get(ctx, types.NamespacedName{Name: webhook.Name, Namespace: webhook.Namespace}, &routev1.Route{})
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Client.Create(ctx, webhook)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to create webhook %v", webhook.Name))
				return "", err
			}
		} else if errors.IsAlreadyExists(err) {
			log.Info("Initial build already exists")
		} else {
			log.Error(err, fmt.Sprintf("Unable to get webhook %v", webhook.Name))
			return "", err
		}
	}

	// Ideally, one must wait for the route to be 'accepted'?
	createdWebhook := &routev1.Route{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: webhook.Name, Namespace: webhook.Namespace}, createdWebhook)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to get inital webhook %v", webhook.Name))
		return "", err
	}

	if createdWebhook != nil && len(createdWebhook.Status.Ingress) != 0 {
		return createdWebhook.Status.Ingress[0].Host, err
	}

	return "", err
}

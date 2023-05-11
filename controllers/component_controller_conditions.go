/*
Copyright 2021-2023 Red Hat, Inc.

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
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
)

func (r *ComponentReconciler) SetCreateConditionAndUpdateCR(ctx context.Context, req ctrl.Request, component *appstudiov1alpha1.Component, createError error) error {
	log := ctrl.LoggerFrom(ctx)

	condition := metav1.Condition{}

	var currentComponent appstudiov1alpha1.Component
	err := r.Get(ctx, req.NamespacedName, &currentComponent)
	if err != nil {
		return nil
	}
	patch := client.MergeFrom(currentComponent.DeepCopy())

	if createError == nil {
		condition = metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Component has been successfully created",
		}
	} else {
		condition = metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Component create failed: %v", createError),
		}
		logutil.LogAPIResourceChangeEvent(log, component.Name, "Component", logutil.ResourceCreate, createError)
	}
	meta.SetStatusCondition(&currentComponent.Status.Conditions, condition)
	currentComponent.Status.Devfile = component.Status.Devfile
	currentComponent.Status.ContainerImage = component.Status.ContainerImage
	currentComponent.Status.GitOps = component.Status.GitOps

	err = r.Client.Status().Patch(ctx, &currentComponent, patch)
	if err != nil {
		// Retry, and if still fails, then return an error
		err = r.Client.Status().Patch(ctx, &currentComponent, patch)
		if err != nil {
			log.Error(err, "Unable to update Component")
		}

		return err
	}
	return nil
}

func (r *ComponentReconciler) SetUpdateConditionAndUpdateCR(ctx context.Context, req ctrl.Request, component *appstudiov1alpha1.Component, updateError error) error {
	log := ctrl.LoggerFrom(ctx)

	condition := metav1.Condition{}
	var currentComponent appstudiov1alpha1.Component
	err := r.Get(ctx, req.NamespacedName, &currentComponent)
	if err != nil {
		return nil
	}
	patch := client.MergeFrom(currentComponent.DeepCopy())
	if updateError == nil {
		condition = metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Component has been successfully updated",
		}
	} else {
		condition = metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Component updated failed: %v", updateError),
		}
		logutil.LogAPIResourceChangeEvent(log, component.Name, "Component", logutil.ResourceUpdate, updateError)
	}

	meta.SetStatusCondition(&component.Status.Conditions, condition)
	currentComponent.Status.Devfile = component.Status.Devfile
	currentComponent.Status.ContainerImage = component.Status.ContainerImage
	currentComponent.Status.GitOps = component.Status.GitOps
	err = r.Client.Status().Patch(ctx, &currentComponent, patch)
	if err != nil {
		// Retry, and if still fails, then return an error
		err = r.Client.Status().Patch(ctx, &currentComponent, patch)
		if err != nil {
			log.Error(err, "Unable to update Component")
		}

		return err
	}

	return nil
}

func (r *ComponentReconciler) SetGitOpsGeneratedConditionAndUpdateCR(ctx context.Context, req ctrl.Request, component *appstudiov1alpha1.Component, generateError error) error {
	log := ctrl.LoggerFrom(ctx)
	condition := metav1.Condition{}
	var currentComponent appstudiov1alpha1.Component
	err := r.Get(ctx, req.NamespacedName, &currentComponent)
	if err != nil {
		return nil
	}
	patch := client.MergeFrom(currentComponent.DeepCopy())
	if generateError == nil {
		condition = metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "GitOps resource generated successfully",
		}
	} else {
		condition = metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionFalse,
			Reason:  "GenerateError",
			Message: fmt.Sprintf("GitOps resources failed to generate: %v", generateError),
		}
		logutil.LogAPIResourceChangeEvent(log, component.Name, "ComponentGitOpsResources", logutil.ResourceCreate, generateError)
	}
	meta.SetStatusCondition(&component.Status.Conditions, condition)
	currentComponent.Status.Devfile = component.Status.Devfile
	currentComponent.Status.ContainerImage = component.Status.ContainerImage
	currentComponent.Status.GitOps = component.Status.GitOps

	err = r.Client.Status().Patch(ctx, &currentComponent, patch)
	if err != nil {
		// Retry, and if still fails, then return an error

		err = r.Client.Status().Patch(ctx, &currentComponent, patch)
		if err != nil {
			log.Error(err, "Unable to update Component")
		}

		return err
	}
	return nil
}

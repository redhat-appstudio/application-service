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
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a ComponentAdapter) SetConditionAndUpdateCR(appErr error) error {
	ctx := a.Ctx
	log := ctrl.LoggerFrom(ctx)
	client := a.NonCachingClient
	component := a.Component

	currentComp := &appstudiov1alpha1.Component{}
	err := client.Get(ctx, a.NamespacedName, currentComp)
	if err != nil {
		return err
	}

	createCond := meta.FindStatusCondition(currentComp.Status.Conditions, "Created")
	var condType, condMessage, reason string
	var condStatus metav1.ConditionStatus

	if createCond != nil && createCond.Status == metav1.ConditionTrue {
		// Set the "Update" status
		condType = "Updated"
		if appErr == nil {
			condMessage = "Component has been successfully updated"
			reason = "OK"
			condStatus = metav1.ConditionTrue
		} else {
			condMessage = fmt.Sprintf("Component update failed: %v", appErr)
			reason = "Error"
			condStatus = metav1.ConditionFalse
		}
	} else {
		condType = "Created"
		if appErr == nil {
			condMessage = "Component has been successfully created"
			reason = "OK"
			condStatus = metav1.ConditionTrue
		} else {
			condMessage = fmt.Sprintf("Component create failed: %v", appErr)
			reason = "Error"
			condStatus = metav1.ConditionFalse
		}
	}

	// Set the status condition
	meta.SetStatusCondition(&currentComp.Status.Conditions, metav1.Condition{
		Type:    condType,
		Status:  condStatus,
		Reason:  reason,
		Message: condMessage,
	})
	logutil.LogAPIResourceChangeEvent(log, currentComp.Name, "Component", logutil.ResourceCreate, appErr)

	currentComp.Status.Devfile = component.Status.Devfile
	currentComp.Status.GitOps = component.Status.GitOps
	currentComp.Status.ContainerImage = component.Status.ContainerImage

	// Update the status of the Component
	err = client.Status().Update(ctx, currentComp)
	if err != nil {
		log.Error(err, "Unable to update Component status")
	}

	return nil
}

func (a ComponentAdapter) SetGitOpsGeneratedConditionAndUpdateCR(generateError error) error {
	ctx := a.Ctx
	log := ctrl.LoggerFrom(ctx)
	client := a.NonCachingClient
	component := a.Component

	currentComp := &appstudiov1alpha1.Component{}
	err := client.Get(ctx, a.NamespacedName, currentComp)
	if err != nil {
		return err
	}

	if generateError == nil {
		meta.SetStatusCondition(&currentComp.Status.Conditions, metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "GitOps resource generated successfully",
		})
	} else {
		meta.SetStatusCondition(&currentComp.Status.Conditions, metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionFalse,
			Reason:  "GenerateError",
			Message: fmt.Sprintf("GitOps resources failed to generate: %v", generateError),
		})
		logutil.LogAPIResourceChangeEvent(log, currentComp.Name, "ComponentGitOpsResources", logutil.ResourceCreate, generateError)
	}

	currentComp.Status.Devfile = component.Status.Devfile
	currentComp.Status.GitOps = component.Status.GitOps
	currentComp.Status.ContainerImage = component.Status.ContainerImage
	err = client.Status().Update(ctx, currentComp)
	if err != nil {
		log.Error(err, "Unable to update Component")
	}
	return nil
}

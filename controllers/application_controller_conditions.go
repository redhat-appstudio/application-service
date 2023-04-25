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

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a ApplicationAdapter) SetConditionAndUpdateCR(appErr error) error {
	ctx := a.Ctx
	log := ctrl.LoggerFrom(ctx)
	client := a.NonCachingClient
	application := a.Application

	currentApp := &appstudiov1alpha1.Application{}
	err := client.Get(ctx, a.NamespacedName, currentApp)
	if err != nil {
		return err
	}

	createCond := meta.FindStatusCondition(currentApp.Status.Conditions, "Created")
	var condType, condMessage, reason string
	var condStatus metav1.ConditionStatus

	if createCond != nil && createCond.Status == metav1.ConditionTrue {
		// Set the "Update" status
		condType = "Updated"
		if appErr == nil {
			condMessage = "Application has been successfully updated"
			reason = "OK"
			condStatus = metav1.ConditionTrue
		} else {
			condMessage = fmt.Sprintf("Application update failed: %v", appErr)
			reason = "Error"
			condStatus = metav1.ConditionFalse
		}
	} else {
		condType = "Created"
		if appErr == nil {
			condMessage = "Application has been successfully created"
			reason = "OK"
			condStatus = metav1.ConditionTrue
		} else {
			condMessage = fmt.Sprintf("Application create failed: %v", appErr)
			reason = "Error"
			condStatus = metav1.ConditionFalse
		}
	}

	// Set the status condition
	meta.SetStatusCondition(&currentApp.Status.Conditions, metav1.Condition{
		Type:    condType,
		Status:  condStatus,
		Reason:  reason,
		Message: condMessage,
	})
	logutil.LogAPIResourceChangeEvent(log, currentApp.Name, "Application", logutil.ResourceCreate, appErr)

	currentApp.Status.Devfile = application.Status.Devfile
	// Update the status of the Application
	err = client.Status().Update(ctx, currentApp)
	if err != nil {
		log.Error(err, "Unable to update Application status")
	}

	return nil
}

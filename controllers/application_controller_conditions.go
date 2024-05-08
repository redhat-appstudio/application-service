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

	logutil "github.com/konflux-ci/application-service/pkg/log"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
)

func (r *ApplicationReconciler) SetCreateConditionAndUpdateCR(ctx context.Context, req ctrl.Request, application *appstudiov1alpha1.Application, createError error) {
	log := ctrl.LoggerFrom(ctx)

	condition := metav1.Condition{}
	var currentApplication appstudiov1alpha1.Application
	err := r.Get(ctx, req.NamespacedName, &currentApplication)
	if err != nil {
		log.Error(err, "Unable to get current Application status")
		return
	}
	patch := client.MergeFrom(currentApplication.DeepCopy())

	if createError == nil {
		condition = metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Application has been successfully created",
		}
	} else {
		condition = metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Application create failed: %v", createError),
		}
		logutil.LogAPIResourceChangeEvent(log, application.Name, "Application", logutil.ResourceCreate, createError)
	}
	meta.SetStatusCondition(&currentApplication.Status.Conditions, condition)
	currentApplication.Status.Devfile = application.Status.Devfile
	err = r.Client.Status().Patch(ctx, &currentApplication, patch)
	if err != nil {
		log.Error(err, "Unable to update Application status")
	}
}

func (r *ApplicationReconciler) SetUpdateConditionAndUpdateCR(ctx context.Context, req ctrl.Request, application *appstudiov1alpha1.Application, updateError error) {
	log := ctrl.LoggerFrom(ctx)
	condition := metav1.Condition{}
	var currentApplication appstudiov1alpha1.Application
	err := r.Get(ctx, req.NamespacedName, &currentApplication)
	if err != nil {
		log.Error(err, "Unable to get current Application status")
		return
	}
	patch := client.MergeFrom(currentApplication.DeepCopy())
	if updateError == nil {
		condition = metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Application has been successfully updated",
		}
	} else {
		condition = metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Application updated failed: %v", updateError),
		}
		logutil.LogAPIResourceChangeEvent(log, application.Name, "Application", logutil.ResourceUpdate, updateError)
	}

	meta.SetStatusCondition(&currentApplication.Status.Conditions, condition)
	currentApplication.Status.Devfile = application.Status.Devfile
	err = r.Client.Status().Patch(ctx, &currentApplication, patch)
	if err != nil {
		log.Error(err, "Unable to update Application status")
	}
}

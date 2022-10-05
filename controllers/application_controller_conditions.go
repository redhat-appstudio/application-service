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
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
)

func (r *ApplicationReconciler) SetCreateConditionAndUpdateCR(ctx context.Context, req ctrl.Request, application *appstudiov1alpha1.Application, createError error) {
	log := r.Log.WithValues("Application", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if createError == nil {
		meta.SetStatusCondition(&application.Status.Conditions, metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Application has been successfully created",
		})
	} else {
		meta.SetStatusCondition(&application.Status.Conditions, metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Application create failed: %v", createError),
		})
	}

	err := r.Client.Status().Update(ctx, application)
	if err != nil {
		log.Error(err, "Unable to update Application")
	}
}

func (r *ApplicationReconciler) SetUpdateConditionAndUpdateCR(ctx context.Context, req ctrl.Request, application *appstudiov1alpha1.Application, updateError error) {
	log := r.Log.WithValues("Application", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if updateError == nil {
		meta.SetStatusCondition(&application.Status.Conditions, metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Application has been successfully updated",
		})
	} else {
		meta.SetStatusCondition(&application.Status.Conditions, metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Application updated failed: %v", updateError),
		})
	}

	err := r.Client.Status().Update(ctx, application)
	if err != nil {
		log.Error(err, "Unable to update Application")
	}
}

/*
Copyright 2022 Red Hat, Inc.

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

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
)

func (r *ComponentDetectionQueryReconciler) SetCreateConditionAndUpdateCR(ctx context.Context, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, createError error) {
	log := r.Log.WithValues("ComponentDetectionQuery", componentDetectionQuery.Name)

	if createError == nil {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "ComponentDetectionQuery has been successfully created",
		})
	} else {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("ComponentDetectionQuery create failed: %v", createError),
		})
	}

	err := r.Client.Status().Update(ctx, componentDetectionQuery)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}

func (r *ComponentDetectionQueryReconciler) SetUpdateConditionAndUpdateCR(ctx context.Context, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, updateError error) {
	log := r.Log.WithValues("ComponentDetectionQuery", componentDetectionQuery.Name)

	if updateError == nil {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "ComponentDetectionQuery has been successfully updated",
		})
	} else {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Updated",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("ComponentDetectionQuery updated failed: %v", updateError),
		})
	}

	err := r.Client.Status().Update(ctx, componentDetectionQuery)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}

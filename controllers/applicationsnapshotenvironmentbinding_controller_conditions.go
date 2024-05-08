//
// Copyright 2022-2023 Red Hat, Inc.
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

package controllers

import (
	"context"
	"fmt"

	logutil "github.com/konflux-ci/application-service/pkg/log"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SnapshotEnvironmentBindingReconciler) SetConditionAndUpdateCR(ctx context.Context, req ctrl.Request, appSnapshotEnvBinding *appstudiov1alpha1.SnapshotEnvironmentBinding, createError error) {
	log := r.Log.WithValues("namespace", req.NamespacedName.Namespace)

	var currentSEB appstudiov1alpha1.SnapshotEnvironmentBinding
	err := r.Get(ctx, req.NamespacedName, &currentSEB)
	if err != nil {
		return
	}

	patch := client.MergeFrom(currentSEB.DeepCopy())
	condition := metav1.Condition{}
	if createError == nil {
		condition = metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "GitOps repository sync successful",
		}
	} else {
		condition = metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionFalse,
			Reason:  "GenerateError",
			Message: fmt.Sprintf("GitOps repository sync failed: %v", createError),
		}

	}
	meta.SetStatusCondition(&currentSEB.Status.GitOpsRepoConditions, condition)
	logutil.LogAPIResourceChangeEvent(log, currentSEB.Name, "SnapshotEnvironmentBinding", logutil.ResourceCreate, createError)
	currentSEB.Status.Components = appSnapshotEnvBinding.Status.Components

	err = r.Client.Status().Patch(ctx, &currentSEB, patch)
	if err != nil {
		log.Error(err, "Unable to update application snapshot environment binding")

	}
}

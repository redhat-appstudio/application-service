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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/util"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
)

func (r *ComponentDetectionQueryReconciler) SetDetectingConditionAndUpdateCR(ctx context.Context, req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery) {
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
		Type:    "Processing",
		Status:  metav1.ConditionTrue,
		Reason:  "Success",
		Message: "ComponentDetectionQuery is processing",
	})

	err := r.Client.Status().Update(ctx, componentDetectionQuery)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}

func (r *ComponentDetectionQueryReconciler) SetCompleteConditionAndUpdateCR(ctx context.Context, req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, completeError error) {
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if completeError == nil {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "ComponentDetectionQuery has successfully finished",
		})
	} else {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("ComponentDetectionQuery failed: %v", completeError),
		})
	}
	updateComponentName(log, ctx, componentDetectionQuery, r.Client)

	err := r.Client.Status().Update(ctx, componentDetectionQuery)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}

func updateComponentName(log logr.Logger, ctx context.Context, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, client client.Client) {
	for key, value := range componentDetectionQuery.Status.ComponentDetected {
		repoUrl := value.ComponentStub.Source.GitSource.URL
		lastElement := repoUrl[strings.LastIndex(repoUrl, "/")+1:]
		repoName := strings.Split(lastElement, ".git")[0]
		componentName := repoName
		context := value.ComponentStub.Source.GitSource.DeepCopy().Context
		if len(componentDetectionQuery.Status.ComponentDetected) > 1 && context != "" && context != "./" {
			componentName = fmt.Sprintf("%s-%s", context, repoName)
		}
		componentName = sanitizeComponentName(log, ctx, componentName, client, componentDetectionQuery.Namespace)
		value.ComponentStub.ComponentName = componentName

		componentDetectionQuery.Status.ComponentDetected[key] = value
	}
}

// sanitizeComponentName sanitizes component name with the following requirements:
// - Contain at most 63 characters
// - Contain only lowercase alphanumeric characters or ‘-’
// - Start with an alphanumeric character
// - End with an alphanumeric character
// - Must not contain all numeric values
func sanitizeComponentName(log logr.Logger, ctx context.Context, name string, client client.Client, namespace string) string {
	err := kvalidation.IsDNS1123Label(name)
	if err != nil {
		exclusive := regexp.MustCompile(`[^a-zA-Z0-9/-]`)
		// filter out invalid characters
		name = exclusive.ReplaceAllString(name, "")

		_, err := strconv.ParseFloat(name, 64)
		if err != nil {
			// convert all Uppercase chars to lowercase
			name = strings.ToLower(name)
		} else {
			// contains only numeric values, prefix a character
			name = fmt.Sprintf("comp-%s", name)
		}
		if len(name) > 58 {
			name = name[0:58]
		}
	}

	// get hc
	hc := &appstudiov1alpha1.Component{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	newErr := client.Get(ctx, namespacedName, hc)

	if (newErr != nil && !apierrors.IsNotFound(newErr)) || !reflect.DeepEqual(*hc, appstudiov1alpha1.Component{}) {
		// name conflict with existing component, append random 4 chars at end of the name
		name = fmt.Sprintf("%s-%s", name, util.GetRandomString(4, true))
	}

	return name
}

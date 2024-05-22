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
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MapComponentToApplication returns an event handler that will convert events on a Component CR to events on its parent Application
func MapComponentToApplication() func(object client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		component := obj.(*appstudiov1alpha1.Component)

		if component != nil && component.Spec.Application != "" {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: component.Namespace,
						Name:      component.Spec.Application,
					},
				},
			}
		}
		// the obj was not in the namespace or it did not have the required Application.
		return []reconcile.Request{}
	}
}

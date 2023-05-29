//
// Copyright 2023 Red Hat, Inc.
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
	"strings"
	"time"

	pactTypes "github.com/pact-foundation/pact-go/types"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func createAppAndComponents(HASAppNamespace string) pactTypes.StateHandler {
	appName := "myapp"
	ghCompName := "gh-component"
	quayCompName := "quay-component"
	ghCompRepoLink := "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	quayRepoLink := "quay.io/test/test-image:latest"

	hasApp := getApplicationSpec(appName, HASAppNamespace)
	ghComp := getGhComponentSpec(ghCompName, HASAppNamespace, appName, ghCompRepoLink)
	quayComp := getQuayComponentSpec(quayCompName, HASAppNamespace, appName, quayRepoLink)

	k8sClient.Create(ctx, hasApp)
	hasAppLookupKey := types.NamespacedName{Name: appName, Namespace: HASAppNamespace}
	createdHasApp := &appstudiov1alpha1.Application{}
	for i := 0; i < 12; i++ {
		k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
		if len(createdHasApp.Status.Conditions) > 0 {
			if createdHasApp.Status.Conditions[0].Type == "Created" {
				break
			}
		}
		time.Sleep(10 * time.Second)
	}

	k8sClient.Create(ctx, ghComp)
	hasCompLookupKey := types.NamespacedName{Name: ghCompName, Namespace: HASAppNamespace}
	createdHasComp := &appstudiov1alpha1.Component{}
	for i := 0; i < 12; i++ {
		k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
		if len(createdHasComp.Status.Conditions) > 1 {
			break
		}
		time.Sleep(10 * time.Second)
	}

	k8sClient.Create(ctx, quayComp)
	hasCompLookupKey2 := types.NamespacedName{Name: quayCompName, Namespace: HASAppNamespace}
	createdHasComp2 := &appstudiov1alpha1.Component{}
	for i := 0; i < 12; i++ {
		k8sClient.Get(context.Background(), hasCompLookupKey2, createdHasComp2)
		if len(createdHasComp2.Status.Conditions) > 1 {
			break
		}
		time.Sleep(10 * time.Second)
	}

	for i := 0; i < 12; i++ {
		k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
		if len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ghCompName) {
			break
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

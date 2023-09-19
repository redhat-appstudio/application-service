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

package contracts

import (
	"context"
	"strings"
	"time"

	gomega "github.com/onsi/gomega"
	models "github.com/pact-foundation/pact-go/v2/models"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

type Comp struct {
	app  AppParams
	repo string
	name string
}

type CompParams struct {
	components []Comp
}

type AppParams struct {
	appName   string
	namespace string
}

const timeout = 10 * time.Second
const interval = 250 * time.Millisecond

// Deprecated
func createAppAndComponents(HASAppNamespace string) models.StateHandler {
	var stateHandler = func(setup bool, s models.ProviderState) (models.ProviderStateResponse, error) {
		if !setup {
			println("skipping state handler")
			return nil, nil
		}

		appName := "myapp"
		ghCompName := "gh-component"
		quayCompName := "quay-component"
		ghCompRepoLink := "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
		quayRepoLink := "quay.io/test/test-image:latest"

		hasApp := getApplicationSpec(appName, HASAppNamespace)
		ghComp := getGhComponentSpec(ghCompName, HASAppNamespace, appName, ghCompRepoLink)
		quayComp := getQuayComponentSpec(quayCompName, HASAppNamespace, appName, quayRepoLink)

		//create app
		gomega.Expect(k8sClient.Create(ctx, hasApp)).Should(gomega.Succeed())
		hasAppLookupKey := types.NamespacedName{Name: appName, Namespace: HASAppNamespace}
		createdHasApp := &appstudiov1alpha1.Application{}
		for i := 0; i < 12; i++ {
			gomega.Expect(k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)).Should(gomega.Succeed())
			if len(createdHasApp.Status.Conditions) > 0 {
				if createdHasApp.Status.Conditions[0].Type == "Created" {
					break
				}
			}
			time.Sleep(10 * time.Second)
		}

		//create gh component
		gomega.Expect(k8sClient.Create(ctx, ghComp)).Should(gomega.Succeed())
		hasCompLookupKey := types.NamespacedName{Name: ghCompName, Namespace: HASAppNamespace}
		createdHasComp := &appstudiov1alpha1.Component{}
		for i := 0; i < 12; i++ {
			gomega.Expect(k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)).Should(gomega.Succeed())
			if len(createdHasComp.Status.Conditions) > 1 {
				break
			}
			time.Sleep(10 * time.Second)
		}
		//create quay component
		gomega.Expect(k8sClient.Create(ctx, quayComp)).Should(gomega.Succeed())
		hasCompLookupKey2 := types.NamespacedName{Name: quayCompName, Namespace: HASAppNamespace}
		createdHasComp2 := &appstudiov1alpha1.Component{}
		for i := 0; i < 12; i++ {
			gomega.Expect(k8sClient.Get(context.Background(), hasCompLookupKey2, createdHasComp2)).Should(gomega.Succeed())
			if len(createdHasComp2.Status.Conditions) > 1 {
				break
			}
			time.Sleep(10 * time.Second)
		}

		for i := 0; i < 12; i++ {
			gomega.Expect(k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)).Should(gomega.Succeed())
			if len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ghCompName) {
				break
			}
			time.Sleep(10 * time.Second)
		}
		return nil, nil
	}
	return stateHandler
}

func createApp() models.StateHandler {
	var stateHandler = func(setup bool, s models.ProviderState) (models.ProviderStateResponse, error) {
		if !setup {
			println("skipping state handler during turndownn phase")
			return nil, nil
		}

		app := parseApp(s.Parameters)
		hasApp := getApplicationSpec(app.appName, app.namespace)

		//create app
		gomega.Expect(k8sClient.Create(ctx, hasApp)).Should(gomega.Succeed())
		hasAppLookupKey := types.NamespacedName{Name: app.appName, Namespace: app.namespace}
		createdHasApp := &appstudiov1alpha1.Application{}

		gomega.Eventually(func() bool {
			k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
			return len(createdHasApp.Status.Conditions) > 0
		}, timeout, interval).Should(gomega.BeTrue())

		return nil, nil
	}
	return stateHandler
}

func createComponents() models.StateHandler {
	var stateHandler = func(setup bool, s models.ProviderState) (models.ProviderStateResponse, error) {
		if !setup {
			println("skipping state handler")
			return nil, nil
		}

		components := parseComp(s.Parameters)

		for _, comp := range components.components {
			ghComp := getGhComponentSpec(comp.name, comp.app.namespace, comp.app.appName, comp.repo)

			hasAppLookupKey := types.NamespacedName{Name: comp.app.appName, Namespace: comp.app.namespace}
			createdHasApp := &appstudiov1alpha1.Application{}

			//create gh component
			gomega.Expect(k8sClient.Create(ctx, ghComp)).Should(gomega.Succeed())
			hasCompLookupKey := types.NamespacedName{Name: comp.name, Namespace: comp.app.namespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			gomega.Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1
			}, timeout, interval).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, comp.name)
			}, timeout, interval).Should(gomega.BeTrue())
		}
		return nil, nil
	}
	return stateHandler
}

func parseApp(params map[string]interface{}) AppParams {
	return AppParams{
		params["params"].(map[string]interface{})["appName"].(string),
		params["params"].(map[string]interface{})["namespace"].(string),
	}
}

func parseComp(params map[string]interface{}) CompParams {
	tmp := params["params"].(map[string]interface{})["components"].([]interface{})
	var components CompParams
	for _, compToParse := range tmp {
		component := compToParse.(map[string]interface{})
		appParsed := AppParams{component["app"].(map[string]interface{})["appName"].(string),
			component["app"].(map[string]interface{})["namespace"].(string)}
		compParsed := Comp{appParsed, component["repo"].(string), component["compName"].(string)}
		components.components = append(components.components, compParsed)
	}
	return components
}

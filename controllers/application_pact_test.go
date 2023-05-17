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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pact-foundation/pact-go/dsl"
	pactTypes "github.com/pact-foundation/pact-go/types"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestContracts(t *testing.T) {

	if os.Getenv("PACT_BROKER_USERNAME") == "" {
		t.Skip("Skipping PACT tests as credentials are not set.")
	}

	RegisterFailHandler(Fail)
	setupTestEnv()

	pact := &dsl.Pact{
		Provider: "HAS",
	}
	pact.LogLevel = "TRACE"

	// Certificate magic - for the mocked service to be able to communicate with kube-apiserver & for authorization
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(testEnv.Config.CAData)
	certs, err := tls.X509KeyPair(testEnv.Config.CertData, testEnv.Config.KeyData)
	if err != nil {
		panic(err)
	}
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{certs},
	}
	// End of certificate magic

	// uncomment for local development and setup path to your pact files folder
	// var pactDir = "/home/us/pacts"
	selector := pactTypes.ConsumerVersionSelector{
		Latest: true,
	}
	HASAppNamespace := "default"
	_, err = pact.VerifyProvider(t, pactTypes.VerifyRequest{
		ProviderVersion:          "1.0.1",
		ConsumerVersionSelectors: []pactTypes.ConsumerVersionSelector{selector},
		ProviderBaseURL:          testEnv.Config.Host,
		// uncomment for local development, comment out all Broker* variables
		// PactURLs:        []string{filepath.ToSlash(fmt.Sprintf("%s/hacdev-has.json", pactDir))},
		BrokerURL:                  "https://pact-broker-hac-pact-broker.apps.hac-devsandbox.5unc.p1.openshiftapps.com",
		BrokerUsername:             os.Getenv("PACT_BROKER_USERNAME"),
		BrokerPassword:             os.Getenv("PACT_BROKER_PASSWORD"),
		CustomTLSConfig:            tlsConfig,
		Verbose:                    true,
		PublishVerificationResults: true,
		StateHandlers: pactTypes.StateHandlers{
			"No app with the name myapp in the default namespace exists.": func() error {
				return nil
			},
			"App myapp exists and has component gh-component and quay-component": func() error {
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
			},
		},
		AfterEach: func() error {
			//Remove all applications and components after each tests
			k8sClient.DeleteAllOf(context.Background(), &appstudiov1alpha1.Application{}, client.InNamespace(HASAppNamespace))
			k8sClient.DeleteAllOf(context.Background(), &appstudiov1alpha1.Component{}, client.InNamespace(HASAppNamespace))
			return nil
		},
	})
	if err != nil {
		t.Errorf("Error while verifying tests. \n %+v", err)
	}

	cancel()
	err = testEnv.Stop()
	if err != nil {
		fmt.Println("Stopping failed")
		fmt.Printf("%+v", err)
		panic("Cleanup failed")
	}

}

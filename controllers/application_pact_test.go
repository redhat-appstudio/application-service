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
	// setup default variables for tests
	HASAppNamespace := "default"

	// setup default variables for Pact
	selector := pactTypes.ConsumerVersionSelector{Branch: "main"}
	verifyRequest := pactTypes.VerifyRequest{
		ProviderVersion:            os.Getenv("COMMIT_SHA"),
		ConsumerVersionSelectors:   []pactTypes.ConsumerVersionSelector{selector},
		BrokerURL:                  "https://pact-broker-hac-pact-broker.apps.hac-devsandbox.5unc.p1.openshiftapps.com",
		Verbose:                    true,
		PublishVerificationResults: false,
	}

	// setup credentianls and publishing
	if os.Getenv("PR_CHECK") == "true" {
		if os.Getenv("PACT_BROKER_USERNAME") == "" {
			t.Skip("Skipping Pact tests from unit test suite during PR check.")
		} else {
			t.Log("Running Pact tests from a PR check.")
			verifyRequest.BrokerUsername = os.Getenv("PACT_BROKER_USERNAME")
			verifyRequest.BrokerPassword = os.Getenv("PACT_BROKER_PASSWORD")
			verifyRequest.PublishVerificationResults = true
			verifyRequest.ProviderTags = []string{"PR" + os.Getenv("PR_NUMBER")}
		}
	} else {
		if os.Getenv("PACT_BROKER_USERNAME") == "" {
			verifyRequest.BrokerUsername = "pactCommonUser"
			verifyRequest.BrokerPassword = "pactCommonPassword123"
			verifyRequest.ProviderVersion = "local"
			t.Log("Running tests locally. Setting up default creds.")

			// For running localy and testing Pacts from a locall directory:
			//  - uncomment pactDir variable and set it to the folder with pacts
			//  - uncomment verifyRequest.PactURLs
			//  - comment out all Broker* variables (BrokerUsername, BrokerPassword, BrokerURL)
			// var pactDir = "/home/us/pacts"
			// verifyRequest.PactURLs = []string{filepath.ToSlash(fmt.Sprintf("%s/HACdev-HAS.json", pactDir))}
		} else {
			t.Skip("Running tests locally with Pact User setup.")
		}
	}

	// Register fail handler and setup test environment (same as during unit tests)
	RegisterFailHandler(Fail)
	setupTestEnv()

	verifyRequest.ProviderBaseURL = testEnv.Config.Host

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
	verifyRequest.CustomTLSConfig = tlsConfig
	// End of certificate magic

	// setup state handlers
	verifyRequest.StateHandlers = pactTypes.StateHandlers{
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
	}
	verifyRequest.AfterEach = func() error {
		//Remove all applications and components after each tests
		k8sClient.DeleteAllOf(context.Background(), &appstudiov1alpha1.Application{}, client.InNamespace(HASAppNamespace))
		k8sClient.DeleteAllOf(context.Background(), &appstudiov1alpha1.Component{}, client.InNamespace(HASAppNamespace))
		return nil
	}

	pact := &dsl.Pact{
		Provider: "HAS",
		LogLevel: "TRACE",
	}

	// Run pact tests
	_, err = pact.VerifyProvider(t, verifyRequest)
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

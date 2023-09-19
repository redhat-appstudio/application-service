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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	models "github.com/pact-foundation/pact-go/v2/models"
	provider "github.com/pact-foundation/pact-go/v2/provider"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestContracts(t *testing.T) {
	// setup default variables for tests
	HASAppNamespace := "default"

	// setup default variables for Pact
	verifyRequest := provider.VerifyRequest{
		Provider:       "HAS",
		RequestTimeout: 60 * time.Second,
		// Default selector should include environments, but as they are not in place yet, using just main branch
		// ConsumerVersionSelectors:   []pactTypes.ConsumerVersionSelector{{Branch: "main"}, {Environment: "stage"}, {Environment: "production"}},
		ConsumerVersionSelectors:   []provider.Selector{&provider.ConsumerVersionSelector{Branch: "main"}},
		ProviderVersion:            os.Getenv("COMMIT_SHA"),
		BrokerURL:                  "https://pact-broker-hac-pact-broker.apps.hac-devsandbox.5unc.p1.openshiftapps.com",
		PublishVerificationResults: false,
		BrokerUsername:             "pactCommonUser",
		BrokerPassword:             "pactCommonPassword123",
	}

	if os.Getenv("SKIP_PACT_TESTS") == "true" {
		t.Skip("Skipping Pact tests as SKIP_PACT_TESTS is set to true.")
	}

	// setup credentials and publishing
	if os.Getenv("PR_CHECK") == "true" {
		if os.Getenv("COMMIT_SHA") == "" {
			t.Skip("Skipping Pact tests from unit test suite during PR check.")
		} else {
			t.Log("Running Pact tests from a PR check. Verifying against main branch and all environments, not pushing results to broker.")
		}
	} else {
		if os.Getenv("PACT_BROKER_USERNAME") == "" {
			t.Log("Running tests locally. Verifying against main branch, not pushing results to broker.")
			verifyRequest.ProviderVersion = "local"
			verifyRequest.ConsumerVersionSelectors = []provider.Selector{&provider.ConsumerVersionSelector{Branch: "main"}}

			// For running localy and testing Pacts from a locall directory:
			//  - uncomment pactDir variable and set it to the folder with pacts
			//  - uncomment verifyRequest.PactURLs
			//  - comment out all Broker* variables (BrokerUsername, BrokerPassword, BrokerURL)
			// var pactDir = "/home/usr/pacts"
			// verifyRequest.PactFiles = []string{filepath.ToSlash(fmt.Sprintf("%s/HACdev-HAS.json", pactDir))}
		} else {
			t.Log("Running tests post-merge. Verifying against main branch and all environments. Pushing results to Pact broker with the branch \"main\".")
			verifyRequest.BrokerUsername = os.Getenv("PACT_BROKER_USERNAME")
			verifyRequest.BrokerPassword = os.Getenv("PACT_BROKER_PASSWORD")
			verifyRequest.ProviderBranch = os.Getenv("PROVIDER_BRANCH")
			verifyRequest.PublishVerificationResults = true
		}
	}

	// Register fail handler and setup test environment (same as during unit tests)
	RegisterFailHandler(Fail)
	k8sClient, testEnv, ctx, cancel = controllers.SetupTestEnv()

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
	verifyRequest.StateHandlers = models.StateHandlers{
		"No app with the name app-to-create in the default namespace exists.": func(setup bool, s models.ProviderState) (models.ProviderStateResponse, error) { return nil, nil },
		// deprecated
		"App myapp exists and has component gh-component and quay-component": createAppAndComponents(HASAppNamespace),
		"Application exists":         createApp(),
		"Application has components": createComponents(),
	}
	verifyRequest.AfterEach = func() error {
		// Remove all applications and components after each tests
		k8sClient.DeleteAllOf(context.Background(), &appstudiov1alpha1.Application{}, client.InNamespace(HASAppNamespace))
		k8sClient.DeleteAllOf(context.Background(), &appstudiov1alpha1.Component{}, client.InNamespace(HASAppNamespace))
		return nil
	}

	// Run pact tests
	err = provider.NewVerifier().VerifyProvider(t, verifyRequest)
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

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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pact-foundation/pact-go/dsl"
	pactTypes "github.com/pact-foundation/pact-go/types"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestContracts(t *testing.T) {
	// setup default variables for tests
	HASAppNamespace := "default"

	// setup default variables for Pact
	verifyRequest := pactTypes.VerifyRequest{
		// Default selector should include environments, but as they are not in place yet, using just main branch
		// ConsumerVersionSelectors:   []pactTypes.ConsumerVersionSelector{{Branch: "main"}, {Environment: "stage"}, {Environment: "production"}},
		ConsumerVersionSelectors:   []pactTypes.ConsumerVersionSelector{{Branch: "main"}},
		ProviderVersion:            os.Getenv("COMMIT_SHA"),
		BrokerURL:                  "https://pact-broker-hac-pact-broker.apps.hac-devsandbox.5unc.p1.openshiftapps.com",
		Verbose:                    true,
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
			verifyRequest.ConsumerVersionSelectors = []pactTypes.ConsumerVersionSelector{{Branch: "main"}}

			// For running localy and testing Pacts from a locall directory:
			//  - uncomment pactDir variable and set it to the folder with pacts
			//  - uncomment verifyRequest.PactURLs
			//  - comment out all Broker* variables (BrokerUsername, BrokerPassword, BrokerURL)
			// var pactDir = "/home/us/pacts"
			// verifyRequest.PactURLs = []string{filepath.ToSlash(fmt.Sprintf("%s/HACdev-HAS.json", pactDir))}
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
		"No app with the name myapp in the default namespace exists.":        func() error { return nil },
		"App myapp exists and has component gh-component and quay-component": <-createAppAndComponents(HASAppNamespace),
	}
	verifyRequest.AfterEach = func() error {
		// Remove all applications and components after each tests
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

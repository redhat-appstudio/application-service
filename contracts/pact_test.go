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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	provider "github.com/pact-foundation/pact-go/v2/provider"
	"github.com/redhat-appstudio/application-service/controllers"
)

func TestContracts(t *testing.T) {
	// Skip tests if "SKIP_PACT_TESTS" env var is set
	// or if the unit tests are running during PR check job
	if os.Getenv("SKIP_PACT_TESTS") == "true" || (os.Getenv("PR_CHECK") == "true" && os.Getenv("COMMIT_SHA") == "") {
		t.Skip("Skipping Pact tests.")
	}

	// Register fail handler and setup test environment (same as during unit tests)
	RegisterFailHandler(Fail)
	k8sClient, testEnv, ctx, cancel = controllers.SetupTestEnv()

	// Create and setup Pact Verifier
	verifyRequest := createVerifier(t)

	// Run pact tests
	err := provider.NewVerifier().VerifyProvider(t, verifyRequest)
	if err != nil {
		t.Errorf("Error while verifying tests. \n %+v", err)
	}

	println("cleanup")
	cleanUpNamespaces()

	cancel()

	err = testEnv.Stop()
	if err != nil {
		fmt.Println("Stopping failed")
		fmt.Printf("%+v", err)
		panic("Cleanup failed")
	}
}

func createVerifier(t *testing.T) provider.VerifyRequest {
	brokerUsername, _ := base64.StdEncoding.DecodeString("cGFjdENvbW1vblVzZXI=")
	brokerPassword, _ := base64.StdEncoding.DecodeString("cGFjdENvbW1vblBhc3N3b3JkMTIz")
	verifyRequest := provider.VerifyRequest{
		Provider:        "HAS",
		RequestTimeout:  60 * time.Second,
		ProviderBaseURL: testEnv.Config.Host,
		// Default selector should include environments, but as they are not in place yet, using just main branch
		ConsumerVersionSelectors:   []provider.Selector{&provider.ConsumerVersionSelector{Branch: "main"}},
		BrokerURL:                  "https://pact-broker-hac-pact-broker.apps.hac-devsandbox.5unc.p1.openshiftapps.com",
		PublishVerificationResults: false,
		BrokerUsername:             string(brokerUsername),
		BrokerPassword:             string(brokerPassword),
		EnablePending:              true,
		ProviderVersion:            "local",
		ProviderBranch:             "main",
	}

	// clean up test env before every test
	verifyRequest.BeforeEach = func() error {
		// workaround for https://github.com/pact-foundation/pact-go/issues/359
		if os.Getenv("SETUP") == "true" {
			return nil
		}
		cleanUpNamespaces()
		return nil
	}

	// setup credentials and publishing
	if os.Getenv("PR_CHECK") != "true" {
		if os.Getenv("PACT_BROKER_USERNAME") == "" {
			// To run Pact tests against local contract files, set LOCAL_PACT_FILES_FOLDER to the folder with pact jsons
			var pactDir, useLocalFiles = os.LookupEnv("LOCAL_PACT_FILES_FOLDER")
			if useLocalFiles {
				verifyRequest.BrokerPassword = ""
				verifyRequest.BrokerUsername = ""
				verifyRequest.BrokerURL = ""
				verifyRequest.PactFiles = []string{filepath.ToSlash(fmt.Sprintf("%s/HACdev-HAS.json", pactDir))}
				t.Log("Running tests locally. Verifying tests from local folder: ", pactDir)
			} else {
				t.Log("Running tests locally. Verifying against main branch, not pushing results to broker.")
				verifyRequest.ConsumerVersionSelectors = []provider.Selector{&provider.ConsumerVersionSelector{Branch: "main"}}
				// to test against changes in specific HAC-dev PR, use Tag:
				// verifyRequest.ConsumerVersionSelectors = []provider.Selector{&provider.ConsumerVersionSelector{Tag: "PR808", Latest: true}}
			}
		} else {
			t.Log("Running tests post-merge. Verifying against main branch and all environments. Pushing results to Pact broker with the branch \"main\".")
			verifyRequest.BrokerUsername = os.Getenv("PACT_BROKER_USERNAME")
			verifyRequest.BrokerPassword = os.Getenv("PACT_BROKER_PASSWORD")
			verifyRequest.ProviderBranch = os.Getenv("PROVIDER_BRANCH")
			verifyRequest.ProviderVersion = os.Getenv("COMMIT_SHA")
			verifyRequest.PublishVerificationResults = true
		}
	}

	// setup state handlers
	verifyRequest.StateHandlers = setupStateHandler()

	// Certificate magic - for the mocked service to be able to communicate with kube-apiserver & for authorization
	verifyRequest.CustomTLSConfig = createTlsConfig()

	return verifyRequest
}

func createTlsConfig() *tls.Config {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(testEnv.Config.CAData)
	certs, err := tls.X509KeyPair(testEnv.Config.CertData, testEnv.Config.KeyData)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{certs},
	}
}

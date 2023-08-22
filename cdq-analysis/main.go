// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("GITHUB_TOKEN must be set as an environment variable")
	}
	gitToken := os.Getenv("GITHUB_TOKEN")

	// Parse all of the possible command-line flags for the tool
	var contextPath, URL, name, devfilePath, dockerfilePath, Revision, namespace, DevfileRegistryURL string
	var isDevfilePresent, isDockerfilePresent, createK8sJob bool
	flag.StringVar(&name, "name", "", "The ComponentDetectionQuery name")
	flag.StringVar(&contextPath, "contextPath", "./", "The context path for the cdq analysis")
	flag.StringVar(&URL, "URL", "", "The URL for the git repository")
	flag.StringVar(&devfilePath, "devfilePath", "", "The devfile path if the devfile present")
	flag.StringVar(&dockerfilePath, "dockerfilePath", "", "The dockerfile path if the dockerfile present")
	flag.StringVar(&Revision, "revision", "", "The revision of the git repo to run cdq analysis against with")
	flag.StringVar(&DevfileRegistryURL, "devfileRegistryURL", pkg.DevfileRegistryEndpoint, "The devfile registry URL")
	flag.StringVar(&namespace, "namespace", "", "The namespace from which to fetch resources")
	flag.BoolVar(&isDevfilePresent, "isDevfilePresent", false, "If the devfile present in the root of the repository")
	flag.BoolVar(&isDockerfilePresent, "isDockerfilePresent", false, "If the dockerfile present in the root of the repository")
	flag.BoolVar(&createK8sJob, "createK8sJob", false, "If a kubernetes job need to be created to send back the result")
	flag.Parse()

	if err := validateVariables(name, URL, namespace, Revision); err != nil {
		log.Fatal(err)
	}

	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("cdq-analysis").WithName("CloneAndAnalyze")
	var ctx context.Context
	var clientset *kubernetes.Clientset
	if createK8sJob {
		ctx = context.Background()
		config, err := rest.InClusterConfig()
		if err != nil {
			fmt.Printf("Error creating InClusterConfig: %v", err)
		}

		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			fmt.Printf("Error creating clientset with config %v: %v", config, err)
		}
	}
	k8sInfoClient := pkg.K8sInfoClient{
		Ctx:          ctx,
		Clientset:    clientset,
		Log:          log,
		CreateK8sJob: createK8sJob,
	}
	pkg.CloneAndAnalyze(k8sInfoClient, gitToken, namespace, name, contextPath, devfilePath, dockerfilePath, URL, Revision, DevfileRegistryURL, isDevfilePresent, isDockerfilePresent)
}

// validateVariables ensures that all of the necessary variables passed in are set to valid values
func validateVariables(name, URL, namespace, revision string) error {

	// The namespace flag must be passed in
	if namespace == "" {
		return fmt.Errorf("usage: --namespace must be set to a Kubernetes namespace")
	}

	// Parse the URL
	if URL == "" {
		return fmt.Errorf("usage: --URL <repository-url> must be passed in as a flag")
	}

	// The name flag must be passed in
	if name == "" {
		return fmt.Errorf("usage: --name <cdq-name> must be passed in as a flag")
	}

	// The revision flag must be passed in
	if revision == "" {
		return fmt.Errorf("usage: --revision <revision> must be passed in as a flag")
	}

	return nil
}

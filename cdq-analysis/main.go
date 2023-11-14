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
	"path/filepath"
	"strconv"

	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	gitToken := os.Getenv("GITHUB_TOKEN")

	// Parse all of the possible command-line flags for the tool
	var contextPath, URL, name, Revision, namespace, DevfileRegistryURL, createK8sJobStr string
	var createK8sJob bool
	flag.StringVar(&name, "name", "", "The ComponentDetectionQuery name")
	flag.StringVar(&contextPath, "contextPath", "./", "The context path for the cdq analysis")
	flag.StringVar(&URL, "URL", "", "The URL for the git repository")
	flag.StringVar(&Revision, "revision", "", "The revision of the git repo to run cdq analysis against with")
	flag.StringVar(&DevfileRegistryURL, "devfileRegistryURL", pkg.DevfileRegistryEndpoint, "The devfile registry URL")
	flag.StringVar(&namespace, "namespace", "", "The namespace from which to fetch resources")
	flag.StringVar(&createK8sJobStr, "createK8sJob", "false", "If a kubernetes job need to be created to send back the result")
	flag.Parse()

	createK8sJob, err := strconv.ParseBool(createK8sJobStr)
	if err != nil {
		log.Fatal(fmt.Errorf("Error parse createK8sJob: %v", err))
		createK8sJob = false
	}

	if err := validateVariables(name, URL, namespace); err != nil {
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
			// Couldn't find an InClusterConfig, may be running outside of Kube, so try to find a local kube config file
			var kubeconfig string
			if os.Getenv("KUBECONFIG") != "" {
				kubeconfig = os.Getenv("KUBECONFIG")
			} else {
				kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
			}
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				fmt.Printf("Error creating clientset with config %v: %v", config, err)
				os.Exit(1)
			}
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

	cdqInfo := &pkg.CDQInfo{
		DevfileRegistryURL: DevfileRegistryURL,
		GitURL:             pkg.GitURL{RepoURL: URL, Revision: Revision, Token: gitToken},
	}

	cdqUtil := pkg.NewCDQUtilClient()

	/* #nosec G104 -- the main.go is triggerred by docker image, and the result as well as the error will be send by the k8s job*/
	pkg.CloneAndAnalyze(k8sInfoClient, namespace, name, contextPath, cdqInfo, cdqUtil)

}

// validateVariables ensures that all of the necessary variables passed in are set to valid values
func validateVariables(name, URL, namespace string) error {

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

	return nil
}

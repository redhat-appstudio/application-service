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
package main

import (
	"context"
	"fmt"
	"go.uber.org/zap/zapcore"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"strings"

	"github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// remove the prefix and suffix quotes
	for i := 1; i <= 10; i++ {
		if strings.HasPrefix(os.Args[i], "\"") && strings.HasSuffix(os.Args[i], "\"") && len(os.Args[i]) > 1 {
			os.Args[i] = os.Args[i][1 : len(os.Args[i])-1]
		}
	}
	gitToken := os.Args[1]
	namespace := os.Args[2]
	name := os.Args[3]
	contextPath := os.Args[4]
	devfilePath := os.Args[5]
	URL := os.Args[6]
	Revision := os.Args[7]
	DevfileRegistryURL := os.Args[8]
	isDevfilePresent, _ := strconv.ParseBool(os.Args[9])
	isDockerfilePresent, _ := strconv.ParseBool(os.Args[10])

	ctx := context.Background()
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error creating InClusterConfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating clientset with config %v: %v", config, err)
	}
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("cdq-analysis").WithName("CloneAndAnalyze")
	k8sInfoClient := pkg.K8sInfoClient{
		Ctx:       ctx,
		Clientset: clientset,
		Log:       log,
	}
	pkg.CloneAndAnalyze(k8sInfoClient, gitToken, namespace, name, contextPath, devfilePath, URL, Revision, DevfileRegistryURL, isDevfilePresent, isDockerfilePresent)
}

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
	"go/build"
	"path/filepath"

	"github.com/redhat-appstudio/application-service/gitops"

	ginkgo "github.com/onsi/ginkgo"
	gomega "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ctrl "sigs.k8s.io/controller-runtime"

	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"github.com/redhat-appstudio/application-service/pkg/devfile"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
)

var (
	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func setupTestEnv() {
	logf.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	applicationAPIDepVersion := "v0.0.0-20230616144210-9dad8e40e3ed"

	ginkgo.By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "hack", "routecrd"),
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "github.com", "redhat-appstudio", "application-api@"+applicationAPIDepVersion, "manifests"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(cfg).NotTo(gomega.BeNil())

	err = appstudiov1alpha1.AddToScheme(scheme.Scheme)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = routev1.AddToScheme(scheme.Scheme)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(k8sClient).NotTo(gomega.BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	mockGhTokenClient := github.MockGitHubTokenClient{}

	// To Do: Set up reconcilers for the other controllers
	err = (&ApplicationReconciler{
		Client:            k8sManager.GetClient(),
		Scheme:            k8sManager.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("Application"),
		GitHubTokenClient: mockGhTokenClient,
		GitHubOrg:         github.AppStudioAppDataOrg,
	}).SetupWithManager(ctx, k8sManager)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = (&ComponentReconciler{
		Client:            k8sManager.GetClient(),
		Scheme:            k8sManager.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("Component"),
		Generator:         gitops.NewMockGenerator(),
		AppFS:             ioutils.NewMemoryFilesystem(),
		SPIClient:         spi.MockSPIClient{},
		GitHubTokenClient: mockGhTokenClient,
	}).SetupWithManager(ctx, k8sManager)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = (&ComponentDetectionQueryReconciler{
		Client:             k8sManager.GetClient(),
		Scheme:             k8sManager.GetScheme(),
		Log:                ctrl.Log.WithName("controllers").WithName("ComponentDetectionQuery"),
		SPIClient:          spi.MockSPIClient{},
		AlizerClient:       devfile.MockAlizerClient{},
		GitHubTokenClient:  mockGhTokenClient,
		DevfileRegistryURL: devfile.DevfileStageRegistryEndpoint, // Use the staging devfile registry for tests
		AppFS:              ioutils.NewMemoryFilesystem(),
	}).SetupWithManager(ctx, k8sManager)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = (&SnapshotEnvironmentBindingReconciler{
		Client:            k8sManager.GetClient(),
		Scheme:            k8sManager.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("SnapshotEnvironmentBinding"),
		Generator:         gitops.NewMockGenerator(),
		AppFS:             ioutils.NewMemoryFilesystem(),
		GitHubTokenClient: mockGhTokenClient,
	}).SetupWithManager(ctx, k8sManager)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	go func() {
		defer ginkgo.GinkgoRecover()
		err = k8sManager.Start(ctx)
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to run manager")
	}()
}

/*
Copyright 2021-2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	spiapi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/konflux-ci/operator-toolkit/webhook"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	routev1 "github.com/openshift/api/route/v1"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/controllers"
	"github.com/redhat-appstudio/application-service/controllers/webhooks"
	"github.com/redhat-appstudio/application-service/pkg/availability"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"

	// Enable pprof for profiling
	/* #nosec G108 -- debug code */
	_ "net/http/pprof"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appstudiov1alpha1.AddToScheme(scheme))

	utilruntime.Must(spiapi.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var apiExportName string
	flag.StringVar(&apiExportName, "api-export-name", "", "The name of the APIExport.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx := ctrl.SetupSignalHandler()

	restConfig := ctrl.GetConfigOrDie()
	setupLog = setupLog.WithValues("controllerKind", apiExportName)

	// Set up pprof if needed
	if os.Getenv("ENABLE_PPROF") == "true" {
		go func() {
			/* #nosec G114 -- debug code */
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}
	var mgr ctrl.Manager
	var err error
	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f50829e1.redhat.com",
		LeaderElectionConfig:   restConfig,
	}
	mgr, err = ctrl.NewManager(restConfig, options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add triggers api to the scheme")
		os.Exit(1)
	}

	// Retrieve the option to specify a custom devfile registry
	devfileRegistryURL := os.Getenv("DEVFILE_REGISTRY_URL")
	if devfileRegistryURL == "" {
		devfileRegistryURL = cdqanalysis.DevfileRegistryEndpoint
	}

	// Retrieve the option to specify a cdq-analysis image
	cdqAnalysisImage := os.Getenv("CDQ_ANALYSIS_IMAGE")
	if cdqAnalysisImage == "" {
		cdqAnalysisImage = cdqanalysis.CDQAnalysisImage
	}

	// Retrieve the option to run cdq analysis with a k8s job
	var RunKubernetesJob bool
	runK8SJobCDQStr := os.Getenv("RUN_K8S_JOB_CDQ")
	if runK8SJobCDQStr == "" {
		RunKubernetesJob = false
	} else {
		RunKubernetesJob, err = strconv.ParseBool(runK8SJobCDQStr)
		if err != nil {
			setupLog.Info(fmt.Sprintf("unable to parse bool value from ENV $RUN_K8S_JOB_CDQ: %v, run go module instead ", err))
			RunKubernetesJob = false
		}
	}

	// Parse any passed in tokens and set up a client for handling the github tokens
	err = github.ParseGitHubTokens()
	if err != nil {
		setupLog.Error(err, "unable to set up github tokens")
		os.Exit(1)
	}
	ghTokenClient := github.GitHubTokenClient{}
	setupLog.Info(fmt.Sprintf("There are %v token(s) available", len(github.Clients)))

	if err = (&controllers.ApplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("Application"),
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Application")
		os.Exit(1)
	}
	if err = (&controllers.ComponentReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("Component"),
		AppFS:             ioutils.NewFilesystem(),
		GitHubTokenClient: ghTokenClient,
		SPIClient: spi.SPIClient{
			K8sClient: mgr.GetClient(),
		},
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Component")
		os.Exit(1)
	}
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
			setupLog.Error(err, "Unable to retrieve Kubernetes InClusterConfig")
			os.Exit(1)
		}
	}
	if err = (&controllers.ComponentDetectionQueryReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                ctrl.Log.WithName("controllers").WithName("ComponentDetectionQuery"),
		GitHubTokenClient:  ghTokenClient,
		DevfileRegistryURL: devfileRegistryURL,
		AppFS:              ioutils.NewFilesystem(),
		CdqAnalysisImage:   cdqAnalysisImage,
		RunKubernetesJob:   RunKubernetesJob,
		Config:             config,
		CDQUtil:            cdqanalysis.NewCDQUtilClient(),
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ComponentDetectionQuery")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		setupLog.Info("setting up webhooks")
		setUpWebhooks(mgr)
	}

	if err = (&controllers.SnapshotEnvironmentBindingReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("SnapshotEnvironmentBinding"),
		AppFS:             ioutils.NewFilesystem(),
		GitHubTokenClient: ghTokenClient,
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SnapshotEnvironmentBinding")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	availabilityChecker := &availability.AvailabilityWatchdog{GitHubTokenClient: ghTokenClient}
	if err := mgr.Add(availabilityChecker); err != nil {
		setupLog.Error(err, "unable to set up availability checks")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// setUpWebhooks sets up webhooks.
func setUpWebhooks(mgr ctrl.Manager) {
	err := webhook.SetupWebhooks(mgr, webhooks.EnabledWebhooks...)
	if err != nil {
		setupLog.Error(err, "unable to setup webhooks")
		os.Exit(1)
	}

	// Retrieve the option to enable HTTP2 on the Webhook server
	enableWebhookHTTP2 := os.Getenv("ENABLE_WEBHOOK_HTTP2")
	if enableWebhookHTTP2 == "" {
		enableWebhookHTTP2 = "false"
	}

	if enableWebhookHTTP2 == "false" {
		setupLog.Info("disabling http/2 on the webhook server")
		server := mgr.GetWebhookServer()
		server.TLSOpts = append(server.TLSOpts,
			func(c *tls.Config) {
				c.NextProtos = []string{"http/1.1"}
			},
		)
	}
}

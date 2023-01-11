package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/generate"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/util"
	gitops "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/redhat-developer/gitops-generator/pkg/util/ioutils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("GITHUB_TOKEN must be set as an environment variable")
	}
	githubToken := os.Getenv("GITHUB_TOKEN")

	// Parse all of the possible command-line flags for the tool
	var operation, repoURL, componentName, sebName, path, branch, namespace string
	flag.StringVar(&componentName, "component", "", "The Component resource name from which to generate the GitOps resources")
	flag.StringVar(&sebName, "seb", "", "The SnapshotEnvrionmentBinding resource name from which to generate the GitOps resources")
	flag.StringVar(&repoURL, "repoURL", "", "The URL for the git repository")
	flag.StringVar(&operation, "operation", "", "The operation to perform. One of: 'generate-base' or 'generate-overlays'")
	flag.StringVar(&branch, "branch", "", "The branch inside the GitOps repository to use. Defaults to main.")
	flag.StringVar(&path, "path", "", "The path within the GitOps repository to use. Defaults to /")
	flag.StringVar(&namespace, "namespace", "", "The namespace from which to fetch resources")
	flag.Parse()

	if err := validateCommandLineFlags(operation, repoURL, namespace, componentName, sebName); err != nil {
		log.Fatal(err)
	}

	kubeClient := initKubeClientOrPanic()

	// Get the remote URL, branch and path for the GitOps repository
	remoteURL, err := util.GetRemoteURL(repoURL, githubToken)
	if err != nil {
		log.Fatal(err)
	}
	if branch == "" {
		branch = "main"
	}
	if path == "" {
		path = "/"
	}

	appFs := ioutils.NewFilesystem()
	if operation == "generate-base" {
		// Parse the Component resource
		component := appstudiov1alpha1.Component{}
		err = kubeClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: componentName}, &component)
		if err != nil {
			log.Fatal(err)
		}

		// Construct the GitOps params and generate the base resources
		gitopsParams := generate.GitOpsGenParams{
			Generator: gitops.NewGitopsGen(),
			RemoteURL: remoteURL,
			Branch:    branch,
			Context:   path,
		}
		err = generate.GenerateGitopsBase(context.Background(), kubeClient, component, appFs, gitopsParams)
		if err == nil {
			log.Fatal(err)
		}
	} else {
		snapshotEnvironmentBinding := appstudiov1alpha1.SnapshotEnvironmentBinding{}
		err = kubeClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: componentName}, &snapshotEnvironmentBinding)
		if err != nil {
			log.Fatal(err)
		}
		gitopsParams := generate.GitOpsGenParams{
			Generator: gitops.NewGitopsGen(),
			RemoteURL: remoteURL,
		}
		err = generate.GenerateGitopsOverlays(context.Background(), kubeClient, snapshotEnvironmentBinding, appFs, gitopsParams)
		if err == nil {
			log.Fatal(err)
		}
	}

}

// initKubeClientOrPanic returns an intialized controller-runtime Kubernetes client with the default Kube and HAS CRD schemes added
// If an error is encountered, it panics
func initKubeClientOrPanic() client.Client {
	// Initialize a Kubeclient. Required for certain operations when generating the .tekton/ resources
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appstudiov1alpha1.AddToScheme(scheme))
	restConfig := ctrl.GetConfigOrDie()
	kubeClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}

	return kubeClient
}

// validateCommandLineFlags ensures that all of the necessary flags passed into the generator are set to valid values
func validateCommandLineFlags(operation, repoURL, namespace, componentName, sebName string) error {
	// If an invalid operation was specified, error out.
	if operation != "generate-base" && operation != "generate-overlays" {
		return fmt.Errorf("usage: --operation must be set to either 'generate-base' or 'generate-overlays'")
	}

	// The namespace flag must be passed in
	if namespace == "" {
		return fmt.Errorf("usage: --namespace must be set to a Kubernetes namespace")
	}

	// Parse the URL
	if repoURL == "" {
		return fmt.Errorf("usage: --repoURL <repository-url> must be passed in as a flag")
	}

	if operation == "generate-base" {
		// Parse the Component resource
		if componentName == "" {
			return fmt.Errorf("usage: --component <component-name> must be passed in as a flag")
		}
	} else {
		if sebName == "" {
			return fmt.Errorf("usage: --seb <seb-name> must be passed in as a flag")
		}
	}

	return nil
}

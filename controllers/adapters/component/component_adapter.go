package component

import (
	"context"
	"fmt"
	"os"

	"github.com/devfile/api/v2/pkg/attributes"
	devfileParser "github.com/devfile/library/v2/pkg/devfile/parser"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Adapter struct {
	Application     *appstudiov1alpha1.Application
	AppFS           afero.Afero
	NamespacedName  types.NamespacedName
	Component       *appstudiov1alpha1.Component
	CompDevfileData data.DevfileData
	Generator       gitopsgen.Generator
	GitHubClient    github.GitHubClient
	SPIClient       spi.SPI
	Client          client.Client
	Ctx             context.Context
	Log             logr.Logger
}

// EnsureComponentDevfile is responsible for ensuring the Component's devfile in the CR status is up to date
func (a *Adapter) EnsureComponentDevfile() (reconciler.OperationResult, error) {
	component := a.Component
	log := a.Log
	ctx := a.Ctx

	source := component.Spec.Source

	var compDevfileData data.DevfileData
	var devfileLocation string
	var devfileBytes []byte
	var err error
	var gitToken string

	if source.GitSource != nil && source.GitSource.URL != "" {
		context := source.GitSource.Context
		// If a Git secret was passed in, retrieve it for use in our Git operations
		// The secret needs to be in the same namespace as the Component
		if component.Spec.Secret != "" {
			gitSecret := corev1.Secret{}
			namespacedName := types.NamespacedName{
				Name:      component.Spec.Secret,
				Namespace: component.Namespace,
			}

			err = a.Client.Get(ctx, namespacedName, &gitSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to retrieve Git secret %v, exiting reconcile loop %v", component.Spec.Secret, a.NamespacedName))
				a.SetConditionAndUpdateCR(err)
				return reconciler.RequeueWithError(err)
			}

			gitToken = string(gitSecret.Data["password"])
		}

		var gitURL string
		if source.GitSource.DevfileURL == "" && source.GitSource.DockerfileURL == "" {
			if gitToken == "" {
				gitURL, err = util.ConvertGitHubURL(source.GitSource.URL, source.GitSource.Revision, context)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", a.NamespacedName))
					a.SetConditionAndUpdateCR(err)
					return reconciler.RequeueWithError(err)
				}

				devfileBytes, devfileLocation, err = devfile.FindAndDownloadDevfile(gitURL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to read the devfile from dir %s %v", gitURL, a.NamespacedName))
					a.SetConditionAndUpdateCR(err)
					return reconciler.RequeueWithError(err)
				}

				devfileLocation = gitURL + string(os.PathSeparator) + devfileLocation
			} else {
				// Use SPI to retrieve the devfile from the private repository
				devfileBytes, err = spi.DownloadDevfileUsingSPI(a.SPIClient, ctx, component.Namespace, source.GitSource.URL, "main", context)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to download from any known devfile locations from %s %v", source.GitSource.URL, a.NamespacedName))
					a.SetConditionAndUpdateCR(err)
					return reconciler.RequeueWithError(err)
				}
			}

		} else if source.GitSource.DevfileURL != "" {
			devfileLocation = source.GitSource.DevfileURL
			devfileBytes, err = util.CurlEndpoint(source.GitSource.DevfileURL)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.GitSource.DevfileURL, a.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.GitSource.DevfileURL)
				a.SetConditionAndUpdateCR(err)
				return reconciler.RequeueWithError(err)
			}
		} else if source.GitSource.DockerfileURL != "" {
			devfileData, err := devfile.CreateDevfileForDockerfileBuild(source.GitSource.DockerfileURL, "./", component.Name, component.Spec.Application, component.Namespace)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to create devfile for dockerfile build %v", a.NamespacedName))
				a.SetConditionAndUpdateCR(err)
				return reconciler.RequeueWithError(err)
			}

			devfileBytes, err = yaml.Marshal(devfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshal devfile, exiting reconcile loop %v", a.NamespacedName))
				a.SetConditionAndUpdateCR(err)
				return reconciler.RequeueWithError(err)
			}
		}
	} else {
		// An image component was specified
		// Generate a stub devfile for the component
		devfileData, err := devfile.ConvertImageComponentToDevfile(*component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to convert the Image Component to a devfile %v", a.NamespacedName))
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
		component.Status.ContainerImage = component.Spec.ContainerImage

		devfileBytes, err = yaml.Marshal(devfileData)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to marshal devfile, exiting reconcile loop %v", a.NamespacedName))
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
	}

	if devfileLocation != "" {
		// Parse the Component Devfile
		devfileSrc := devfile.DevfileSrc{
			URL: devfileLocation,
		}
		compDevfileData, err = devfile.ParseDevfile(devfileSrc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component devfile location, exiting reconcile loop %v", a.NamespacedName))
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
	} else {
		// Parse the Component Devfile
		devfileSrc := devfile.DevfileSrc{
			Data: string(devfileBytes),
		}
		compDevfileData, err = devfile.ParseDevfile(devfileSrc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", a.NamespacedName))
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
	}

	err = a.updateComponentDevfileModel(compDevfileData, *component)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", a.NamespacedName))
		a.SetConditionAndUpdateCR(err)
		return reconciler.RequeueWithError(err)
	}

	yamlHASCompData, err := yaml.Marshal(compDevfileData)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to marshall the Component devfile, exiting reconcile loop %v", a.NamespacedName))
		a.SetConditionAndUpdateCR(err)
		return reconciler.RequeueWithError(err)
	}

	a.CompDevfileData = compDevfileData
	// Set the devfile and container image in the status
	component.Status.Devfile = string(yamlHASCompData)
	component.Status.ContainerImage = component.Spec.ContainerImage
	return reconciler.ContinueProcessing()
}

// EnsureComponentGitOpsResources is reponsible for ensuring that the Component's GitOps resources get generated
func (a *Adapter) EnsureComponentGitOpsResources() (reconciler.OperationResult, error) {
	component := a.Component
	log := a.Log

	var err error

	devfileSrc := devfile.DevfileSrc{
		Data: a.Application.Status.Devfile,
	}
	devfileData, err := devfile.ParseDevfile(devfileSrc)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to parse the devfile from Application, exiting reconcile loop %v", a.NamespacedName))
		a.SetConditionAndUpdateCR(err)
		return reconciler.RequeueWithError(err)
	}

	log.Info(fmt.Sprintf("Adding the GitOps repository information to the status for component %v", a.NamespacedName))
	err = setGitopsStatus(a.Component, devfileData)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to retrieve gitops repository information for resource %v", a.NamespacedName))
		a.SetConditionAndUpdateCR(err)
		return reconciler.RequeueWithError(err)
	}

	// Generate and push the gitops resources
	if !component.Spec.SkipGitOpsResourceGeneration {
		if err := a.generateGitops(a.Component, a.CompDevfileData); err != nil {
			errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", a.NamespacedName)
			log.Error(err, errMsg)
			a.SetGitOpsGeneratedConditionAndUpdateCR(fmt.Errorf("%v: %v", errMsg, err))
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		} else {
			a.SetGitOpsGeneratedConditionAndUpdateCR(nil)
		}
	}
	return reconciler.ContinueProcessing()
}

// EnsureApplicationStatus ensures that the status of the Application gets updated to 'Created/Updated'
func (a *Adapter) EnsureComponentStatus() (reconciler.OperationResult, error) {
	a.SetConditionAndUpdateCR(nil)
	return reconciler.ContinueProcessing()
}

// generateGitops retrieves the necessary information about a Component's gitops repository (URL, branch, context)
// and attempts to use the GitOps package to generate gitops resources based on that component
func (a *Adapter) generateGitops(component *appstudiov1alpha1.Component, compDevfileData data.DevfileData) error {
	componentName := component.Name
	log := a.Log
	ctx := a.Ctx
	ghClient := a.GitHubClient

	gitOpsURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(component.Status.GitOps, ghClient.Token)
	if err != nil {
		return err
	}

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, a.AppFS)
	if err != nil {
		log.Error(err, "unable to create temp directory for GitOps resources due to error")
		return fmt.Errorf("unable to create temp directory for GitOps resources due to error: %v", err)
	}

	deployAssociatedComponents, err := devfileParser.GetDeployComponents(compDevfileData)
	if err != nil {
		log.Error(err, "unable to get deploy components")
		return err
	}

	kubernetesResources, err := devfile.GetResourceFromDevfile(log, compDevfileData, deployAssociatedComponents, component.Name, component.Spec.Application, component.Spec.ContainerImage, component.Namespace)
	if err != nil {
		log.Error(err, "unable to get kubernetes resources from the devfile outerloop components")
		return err
	}

	// Generate and push the gitops resources
	mappedGitOpsComponent := util.GetMappedGitOpsComponent(*component, kubernetesResources)

	//add the token name to the metrics.  When we add more tokens and rotate, we can determine how evenly distributed the requests are
	metrics.ControllerGitRequest.With(prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "CloneGenerateAndPush"}).Inc()
	err = a.Generator.CloneGenerateAndPush(tempDir, gitOpsURL, mappedGitOpsComponent, a.AppFS, gitOpsBranch, gitOpsContext, false)
	if err != nil {
		log.Error(err, "unable to generate gitops resources due to error")
		return err
	}

	//Gitops functions return sanitized error messages
	metrics.ControllerGitRequest.With(prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "CommitAndPush"}).Inc()
	err = a.Generator.CommitAndPush(tempDir, "", gitOpsURL, mappedGitOpsComponent.Name, gitOpsBranch, "Generating GitOps resources")
	if err != nil {
		log.Error(err, "unable to commit and push gitops resources due to error")
		return err
	}

	// Get the commit ID for the gitops repository
	var commitID string
	repoName, orgName, err := github.GetRepoAndOrgFromURL(gitOpsURL)
	if err != nil {
		gitOpsErr := &util.GitOpsParseRepoError{RemoteURL: gitOpsURL, Err: err}
		log.Error(gitOpsErr, "")
		return gitOpsErr
	}

	metricsLabel := prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "GetLatestCommitSHAFromRepository"}
	metrics.ControllerGitRequest.With(metricsLabel).Inc()
	commitID, err = ghClient.GetLatestCommitSHAFromRepository(ctx, repoName, orgName, gitOpsBranch)
	metrics.HandleRateLimitMetrics(err, metricsLabel)
	if err != nil {
		gitOpsErr := &util.GitOpsCommitIdError{Err: err}
		log.Error(gitOpsErr, "")
		return gitOpsErr
	}
	component.Status.GitOps.CommitID = commitID

	// Remove the temp folder that was created
	return a.AppFS.RemoveAll(tempDir)
}

// setGitopsStatus adds the necessary gitops info (url, branch, context) to the component CR status
func setGitopsStatus(component *appstudiov1alpha1.Component, devfileData data.DevfileData) error {
	// Get the devfile of the Application CR
	var err error

	devfileAttributes := devfileData.GetMetadata().Attributes

	// Get the GitOps repository URL
	gitOpsURL := devfileAttributes.GetString("gitOpsRepository.url", &err)
	if err != nil {
		return fmt.Errorf("unable to retrieve GitOps repository from Application CR devfile: %v", err)
	}
	component.Status.GitOps.RepositoryURL = gitOpsURL

	// Get the GitOps repository branch
	gitOpsBranch := devfileAttributes.GetString("gitOpsRepository.branch", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return err
		}
	}
	if gitOpsBranch != "" {
		component.Status.GitOps.Branch = gitOpsBranch
	}

	// Get the GitOps repository context
	gitOpsContext := devfileAttributes.GetString("gitOpsRepository.context", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return err
		}
	}
	if gitOpsContext != "" {
		component.Status.GitOps.Context = gitOpsContext
	}

	component.Status.GitOps.ResourceGenerationSkipped = component.Spec.SkipGitOpsResourceGeneration
	return nil
}

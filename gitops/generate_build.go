/*
Copyright 2021 Red Hat, Inc.

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

package gitops

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/redhat-developer/gitops-generator/pkg/resources"
	"github.com/redhat-developer/gitops-generator/pkg/yaml"

	devfilev1alpha2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfilecommon "github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	gitopsprepare "github.com/redhat-appstudio/application-service/gitops/prepare"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/spf13/afero"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	triggersapi "github.com/tektoncd/triggers/pkg/apis/triggers/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	buildCommonStoragePVCFileName = "common-storage-pvc.yaml"
	buildTriggerTemplateFileName  = "trigger-template.yaml"
	buildEventListenerFileName    = "event-listener.yaml"
	buildWebhookRouteFileName     = "build-webhook-route.yaml"
	buildRepositoryFileName       = "pac-repository.yaml"

	DefaultImageRepo = "quay.io/redhat-appstudio/user-workload"

	PaCAnnotation                     = "pipelinesascode"
	GitProviderAnnotationName         = "git-provider"
	PipelinesAsCodeWebhooksSecretName = "pipelines-as-code-webhooks-secret"
	PipelinesAsCode_githubAppIdKey    = "github-application-id"
	PipelinesAsCode_githubPrivateKey  = "github-private-key"
)

var (
	imageRegistry = DefaultImageRepo
)

func GetDefaultImageRepo() string {
	return imageRegistry
}

func SetDefaultImageRepo(repo string) {
	imageRegistry = repo
}

func GenerateBuild(fs afero.Fs, outputFolder string, component appstudiov1alpha1.Component, gitopsConfig gitopsprepare.GitopsConfig) error {
	var buildResources map[string]interface{}
	val, ok := component.Annotations[PaCAnnotation]
	if (ok && val == "1") || gitopsConfig.IsHACBS {
		repository, err := GeneratePACRepository(component, gitopsConfig.PipelinesAsCodeCredentials)
		if err != nil {
			return err
		}
		buildResources = map[string]interface{}{
			buildRepositoryFileName: repository,
		}
	} else {
		triggerTemplate, err := GenerateTriggerTemplate(component, gitopsConfig)
		if err != nil {
			return err
		}
		eventListener := GenerateEventListener(component, *triggerTemplate)
		webhookRoute := GenerateBuildWebhookRoute(component)

		buildResources = map[string]interface{}{
			//buildCommonStoragePVCFileName: commonStoragePVC,
			buildTriggerTemplateFileName: triggerTemplate,
			buildEventListenerFileName:   eventListener,
			buildWebhookRouteFileName:    webhookRoute,
		}
	}

	kustomize := resources.Kustomization{}
	for fileName := range buildResources {
		kustomize.AddResources(fileName)
	}

	buildResources[kustomizeFileName] = kustomize

	if _, err := yaml.WriteResources(fs, outputFolder, buildResources); err != nil {
		return err
	}
	return nil
}

// GenerateInitialBuildPipelineRun generates pipeline run for initial build of the component.
func GenerateInitialBuildPipelineRun(component appstudiov1alpha1.Component, gitopsConfig gitopsprepare.GitopsConfig) (tektonapi.PipelineRun, error) {
	// normalizeOutputImageURL is not called with initial builds so we can ignore the error here
	params, err := getParamsForComponentBuild(component, true)
	if err != nil {
		return tektonapi.PipelineRun{}, err
	}
	initialBuildSpec := DetermineBuildExecution(component, params, getInitialBuildWorkspaceSubpath(), gitopsConfig)

	return tektonapi.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: component.Name + "-",
			Namespace:    component.Namespace,
			Labels:       getBuildCommonLabelsForComponent(&component),
		},
		Spec: initialBuildSpec,
	}, nil
}

func getInitialBuildWorkspaceSubpath() string {
	return "initialbuild-" + time.Now().Format("2006-Jan-02_15-04-05")
}

// DetermineBuildExecution returns the pipelineRun spec that would be used
// in webhooks-triggered pipelineRuns as well as user-triggered PipelineRuns
func DetermineBuildExecution(component appstudiov1alpha1.Component, params []tektonapi.Param, workspaceSubPath string, gitopsConfig gitopsprepare.GitopsConfig) tektonapi.PipelineRunSpec {

	pipelineRunSpec := tektonapi.PipelineRunSpec{
		Params: params,
		PipelineRef: &tektonapi.PipelineRef{
			Name:   determineBuildPipeline(component),
			Bundle: gitopsConfig.BuildBundle,
		},

		Workspaces: []tektonapi.WorkspaceBinding{
			{
				Name: "workspace",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "appstudio",
				},
				SubPath: component.Name + "/" + workspaceSubPath,
			},
		},
	}
	if gitopsConfig.AppStudioRegistrySecretPresent {
		pipelineRunSpec.Workspaces = append(pipelineRunSpec.Workspaces, tektonapi.WorkspaceBinding{
			Name: "registry-auth",
			Secret: &corev1.SecretVolumeSource{
				SecretName: "redhat-appstudio-registry-pull-secret",
			},
		})
	}
	return pipelineRunSpec
}

// determineBuildPipeline should detect build pipeline to use for the component and return its name.
// If it fails to autodetect right pipeline, noop pipeline will be returned.
// If a repository consists of two parts (e.g. frontend and backend), it should be mapped to two components (see context field in CR).
// Available build pipeleines are located here: https://github.com/redhat-appstudio/build-definitions/tree/main/pipelines
func determineBuildPipeline(component appstudiov1alpha1.Component) string {
	// It is possible to skip error checks here because the model is propogated by component controller
	componentDevfileData, err := devfile.ParseDevfileModel(component.Status.Devfile)
	if err != nil {
		// Return dummy value for tests
		return "noop"
	}

	// Check for Dockerfile
	filterOptions := devfilecommon.DevfileOptions{
		ComponentOptions: devfilecommon.ComponentOptions{
			ComponentType: devfilev1alpha2.ImageComponentType,
		},
	}
	devfileComponents, _ := componentDevfileData.GetComponents(filterOptions)
	for _, devfileComponent := range devfileComponents {
		if devfileComponent.Image != nil && devfileComponent.Image.Dockerfile != nil {
			return "docker-build"
		}
	}

	// The only information about project is in language and projectType fileds under metadata of the devfile.
	// They must be used to determine the right build pipeline.
	devfileMetadata := componentDevfileData.GetMetadata()
	language := devfileMetadata.Language
	// TODO use projectType when build pipelines support frameworks
	// projectType := devfileMetadata.ProjectType

	switch strings.ToLower(language) {
	case "java":
		return "java-builder"
	case "nodejs", "node":
		return "nodejs-builder"
	case "python":
		// TODO return python-builder when the pipeline ready
		return "noop"
	}

	// Failed to detect build pipeline
	// Do nothing as we do not know how to build given component
	return "noop"
}

func protectDefaultImageRepo(outputImage, namespace string) error {
	// do not allow use of the default registry and a different user's tag
	if strings.HasPrefix(outputImage, GetDefaultImageRepo()) {
		if !strings.HasPrefix(outputImage, GetDefaultImageRepo()+":"+namespace+"-") || !strings.Contains(outputImage, ":") {
			return fmt.Errorf("invalid user image tag combination of default repo %s and component namespace %s", outputImage, namespace)
		}
	}

	return nil
}

func normalizeOutputImageURL(outputImage string) string {
	// Check if the image has commit SHA suffix and delete it if so
	shaSuffixRegExp := regexp.MustCompile(`(.+)-[0-9a-f]{40}$`)
	foundImage := shaSuffixRegExp.FindSubmatch([]byte(outputImage))
	if foundImage != nil {
		// The outputImage matches regexp above and has format: image-sha1234
		// Do not use the suffix in gitops repository.
		outputImage = string(foundImage[1])
	}

	// If provided image has a tag, then append dash and git revision to it.
	// Otherwise, use git revision as the tag.
	// Examples:
	//   quay.io/foo/bar:mytag ==> quay.io/foo/bar:mytag-git-revision
	//   quay.io/foo/bar       ==> quay.io/foo/bar:latest-git-revision
	if strings.Contains(outputImage, ":") {
		outputImage = outputImage + "-" + "$(tt.params.git-revision)"
	} else {
		outputImage = outputImage + ":latest-" + "$(tt.params.git-revision)"
	}
	return outputImage
}

// getParamsForComponentBuild would return the 'input' parameters for the PipelineRun
// that would build an image from source of the Component.
// The key difference between webhook (regular) triggered PipelineRuns and user-triggered (initial) PipelineRuns
// is that the git revision appended to the output image tag in case of webhook build.
func getParamsForComponentBuild(component appstudiov1alpha1.Component, isInitialBuild bool) ([]tektonapi.Param, error) {
	sourceCode := component.Spec.Source.GitSource.URL
	revision := component.Spec.Source.GitSource.Revision
	outputImage := component.Spec.ContainerImage
	var err error

	if err = protectDefaultImageRepo(outputImage, component.Namespace); err != nil {
		return []tektonapi.Param{}, err
	}

	if !isInitialBuild {
		outputImage = normalizeOutputImageURL(component.Spec.ContainerImage)
	}

	// Default required parameters
	params := []tektonapi.Param{
		{
			Name: "git-url",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: sourceCode,
			},
		},
		{
			Name: "output-image",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: outputImage,
			},
		},
	}
	// if revision is specified in the component
	// use it in the parms to the Pipeline Run
	if revision != "" {
		params = append(params, tektonapi.Param{
			Name: "revision",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: revision,
			},
		})
	}
	// Analyze component model for additional parameters
	dockerfile, err := devfile.SearchForDockerfile([]byte(component.Status.Devfile))
	if err != nil {
		return []tektonapi.Param{}, err
	}
	if dockerfile != nil {
		// Set dockerfile location
		params = append(params, tektonapi.Param{
			Name: "dockerfile",
			Value: tektonapi.ArrayOrString{
				Type:      tektonapi.ParamTypeString,
				StringVal: dockerfile.Uri,
			},
		})

		// Check for docker build context
		if dockerfile.BuildContext != "" {
			params = append(params, tektonapi.Param{
				Name: "path-context",
				Value: tektonapi.ArrayOrString{
					Type:      tektonapi.ParamTypeString,
					StringVal: dockerfile.BuildContext,
				},
			})
		}
	}

	return params, err
}

func getBuildCommonLabelsForComponent(component *appstudiov1alpha1.Component) map[string]string {
	labels := map[string]string{
		"pipelines.appstudio.openshift.io/type":    "build",
		"build.appstudio.openshift.io/build":       "true",
		"build.appstudio.openshift.io/type":        "build",
		"build.appstudio.openshift.io/version":     "0.1",
		"build.appstudio.openshift.io/component":   component.Name,
		"build.appstudio.openshift.io/application": component.Spec.Application,
	}
	return labels
}

// GenerateBuildWebhookRoute returns the Route resource that would enable
// ingress traffic into the webhook endpoint ( aka EventListener)
// TODO: This needs to be secure.
func GenerateBuildWebhookRoute(component appstudiov1alpha1.Component) routev1.Route {
	var port int32 = 8080
	webhook := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: "route.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "el" + component.Name,
			Namespace:   component.Namespace,
			Annotations: getBuildCommonLabelsForComponent(&component),
		},
		Spec: routev1.RouteSpec{
			Path: "/",
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "el-" + component.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{IntVal: port},
			},
		},
	}
	return *webhook
}

// GenerateTriggerTemplate generates the TriggerTemplate resources
// which defines how a webhook-based trigger event would be handled -
// In this case, a PipelineRun to build an image would be created.
func GenerateTriggerTemplate(component appstudiov1alpha1.Component, gitopsConfig gitopsprepare.GitopsConfig) (*triggersapi.TriggerTemplate, error) {
	params, err := getParamsForComponentBuild(component, false)
	if err != nil {
		return nil, err
	}
	webhookBasedBuildTemplate := DetermineBuildExecution(component, params, "$(tt.params.git-revision)", gitopsConfig)
	resoureTemplatePipelineRun := tektonapi.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: component.Name + "-",
			Namespace:    component.Namespace,
			Annotations:  getBuildCommonLabelsForComponent(&component),
			Labels:       getBuildCommonLabelsForComponent(&component),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineRun",
			APIVersion: "tekton.dev/v1beta1",
		},
		Spec: webhookBasedBuildTemplate,
	}
	resourceTemplatePipelineRunBytes, err := json.Marshal(resoureTemplatePipelineRun)
	if err != nil {
		return nil, err
	}
	triggerTemplate := triggersapi.TriggerTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TriggerTemplate",
			APIVersion: "triggers.tekton.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
		},
		Spec: triggersapi.TriggerTemplateSpec{
			Params: []triggersapi.ParamSpec{
				{
					Name: "git-revision",
				},
			},
			ResourceTemplates: []triggersapi.TriggerResourceTemplate{
				{
					RawExtension: runtime.RawExtension{Raw: resourceTemplatePipelineRunBytes},
				},
			},
		},
	}
	return &triggerTemplate, err
}

// The GenerateEventListener is responsible for defining how to "parse" the incoming event ( "github-push ")
// and create the resultant PipelineRun ( defined as a TriggerTemplate ).
// The reconciler for EventListeners create a Service, which when exposed enables
// ingress traffic from Github events.
func GenerateEventListener(component appstudiov1alpha1.Component, triggerTemplate triggersapi.TriggerTemplate) triggersapi.EventListener {
	eventListener := triggersapi.EventListener{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EventListener",
			APIVersion: "triggers.tekton.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: getBuildCommonLabelsForComponent(&component),
		},
		Spec: triggersapi.EventListenerSpec{
			ServiceAccountName: "pipeline",
			Triggers: []triggersapi.EventListenerTrigger{
				{
					Bindings: []*triggersapi.TriggerSpecBinding{
						{
							Ref:  "github-push",
							Kind: triggersapi.ClusterTriggerBindingKind,
						},
					},
					Template: &triggersapi.TriggerSpecTemplate{
						Ref: &triggerTemplate.Name,
					},
				},
			},
		},
	}
	return eventListener
}

// GeneratePACRepository creates configuration of Pipelines as Code repository object.
func GeneratePACRepository(component appstudiov1alpha1.Component, config map[string][]byte) (*pacv1alpha1.Repository, error) {
	gitProvider, err := GetGitProvider(component)
	if err != nil {
		return nil, err
	}

	isAppUsed := IsPaCApplicationConfigured(gitProvider, config)

	var gitProviderConfig *pacv1alpha1.GitProvider = nil
	if !isAppUsed {
		// Webhook is used
		gitProviderConfig = &pacv1alpha1.GitProvider{
			Secret: &pacv1alpha1.Secret{
				Name: gitopsprepare.PipelinesAsCodeSecretName,
				Key:  GetProviderTokenKey(gitProvider),
			},
			WebhookSecret: &pacv1alpha1.Secret{
				Name: PipelinesAsCodeWebhooksSecretName,
				Key:  GetWebhookSecretKeyForComponent(component),
			},
		}

		if gitProvider == "gitlab" {
			gitProviderConfig.URL = "https://gitlab.com"
		}
	}

	repository := &pacv1alpha1.Repository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Repository",
			APIVersion: "pipelinesascode.tekton.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: getBuildCommonLabelsForComponent(&component),
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL:         component.Spec.Source.GitSource.URL,
			GitProvider: gitProviderConfig,
		},
	}

	return repository, nil
}

// GetProviderTokenKey returns key (field name) of the given provider access token in the Pipelines as Code k8s secret
func GetProviderTokenKey(gitProvider string) string {
	return gitProvider + ".token"
}

func GetWebhookSecretKeyForComponent(component appstudiov1alpha1.Component) string {
	gitRepoUrl := strings.TrimSuffix(component.Spec.Source.GitSource.URL, ".git")

	notAllowedCharRegex, _ := regexp.Compile("[^-._a-zA-Z0-9]{1}")
	return notAllowedCharRegex.ReplaceAllString(gitRepoUrl, "_")
}

// GetGitProvider returns git provider name based on the repository url, e.g. github, gitlab, etc or git-privider annotation
func GetGitProvider(component appstudiov1alpha1.Component) (string, error) {
	allowedGitProviders := map[string]bool{"github": true, "gitlab": true, "bitbucket": true}
	gitProvider := ""

	sourceUrl := component.Spec.Source.GitSource.URL
	u, err := url.Parse(sourceUrl)
	if err != nil {
		return "", err
	}
	uParts := strings.Split(u.Hostname(), ".")
	if len(uParts) == 1 {
		gitProvider = uParts[0]
	} else {
		gitProvider = uParts[len(uParts)-2]
	}

	if !allowedGitProviders[gitProvider] {
		// Self-hosted git provider, check for git-provider annotation on the component
		gitProviderAnnotationValue := component.GetAnnotations()[GitProviderAnnotationName]
		if gitProviderAnnotationValue != "" {
			if allowedGitProviders[gitProviderAnnotationValue] {
				gitProvider = gitProviderAnnotationValue
			} else {
				err = fmt.Errorf("unsupported \"%s\" annotation value: %s", GitProviderAnnotationName, gitProviderAnnotationValue)
			}
		} else {
			err = fmt.Errorf("self-hosted git provider is not specified via \"%s\" annotation in the component", GitProviderAnnotationName)
		}
	}

	return gitProvider, err
}

// IsPaCApplicationConfigured checks if Pipelines as Code credentials configured for given provider.
// Application is preffered over webhook if possible.
func IsPaCApplicationConfigured(gitProvider string, config map[string][]byte) bool {
	isAppUsed := false

	switch gitProvider {
	case "github":
		if len(config[PipelinesAsCode_githubAppIdKey]) != 0 || len(config[PipelinesAsCode_githubPrivateKey]) != 0 {
			isAppUsed = true
		}
	default:
		// Application is not supported
		isAppUsed = false
	}

	return isAppUsed
}

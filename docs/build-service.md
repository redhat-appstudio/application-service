# Setting up the AppStudio Build Service environment

* Install `OpenShift GitOps` from the in-cluster Operator Marketplace.
* `oc -n openshift-gitops apply -f https://raw.githubusercontent.com/redhat-appstudio/infra-deployments/main/argo-cd-apps/base/build.yaml`

As a user, upon creation of Component, Tekton resources would be created by the controller.

To use auto generated image repository for the Component's image add `image.redhat.com/generate: "true"` annotation to the Component.

If you wish to get 'working' PipelineRuns with user provided image repository, create an image pull secret and link it to the `pipeline` Service Account in the Component's namespace (in both `secrets` and `imagePullSecrets` sections).
See [Kubernetes docs](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials) for more information on how to create `Secrets` containing registry credentials.
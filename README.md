# Hybrid Application Service (HAS)

[![codecov](https://codecov.io/gh/redhat-appstudio/application-service/branch/main/graph/badge.svg)](https://codecov.io/gh/redhat-appstudio/application-service)


A Kubernetes operator to create and manage applications and control the lifecycle of applications.


## Building & Testing
This operator provides a `Makefile` to run all the usual development tasks. If you simply run `make` without any arguments, you'll get a list of available "targets".

To build the operator binary run:

```
make build
```

To test the code:

```
make test
```

To build the docker image of the operator one can run:

```
make docker-build
```

This will make a docker image called `controller:latest` which might or might not be what you want. To override the name of the image build, specify it in the `IMG` environment variable, e.g.:

```
IMG=quay.io/user/hasoperator:next make docker-build
```

To push the image to an image repository one can use:

```
make docker-push
```

The image being pushed can again be modified using the environment variable:
```
IMG=quay.io/user/hasoperator:next make docker-push
```

## Deploying the Operator (non-KCP)

The following section outlines the steps to deploy HAS on a physical Kubernetes cluster. If you are looking to deploy HAS on KCP, please see [this document](./docs/kcp.md).

### Setting up the AppStudio Build Service environment

* Install `OpenShift GitOps` from the in-cluster Operator Marketplace.
* `oc -n openshift-gitops apply -f https://raw.githubusercontent.com/redhat-appstudio/infra-deployments/main/argo-cd-apps/base/build.yaml`

As a user, upon creation of Component, Tekton resources would be created by the controller. 

If you wish to get 'working' PipelineRuns,
* Create an image pull secret named `redhat-appstudio-registry-pull-secret`. See [Kubernetes docs](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials) for more information on how to create
`Secrets` containing registry credentials. 
* Configure the default image repository to which Pipelines would push images to by defining the environment variable `IMAGE_REPOSITORY` for the operator deployment.
Defaults to `quay.io/redhat-appstudio/user-workload`.

Pipelines would use the credentials in the image pull secret `redhat-appstudio-registry-pull-secret` to push to $IMAGE_REPOSITORY.



### Creating a GitHub Secret for HAS

Before deploying the operator, you must ensure that a secret, `has-github-token`, exists in the namespace where HAS will be deployed. This secret must contain a key-value pair, where the key is `token` and where the value points to a valid GitHub Personal Access Token.

The token that is used here must have the following permissions set:
- `repo`
- `delete_repo`

In addition to this, the GitHub token must be associated with an account that has write access to the GitHub organization you plan on using with HAS (see next section).

For example, on OpenShift:
<img width="862" alt="Screen Shot 2021-12-14 at 1 08 43 AM" src="https://user-images.githubusercontent.com/6880023/145942734-63422532-6fad-4017-9d26-79436fe241b8.png">

### Using Private Git Repos

HAS requires SPI to be set up in order to work with private git repositories.

See [private-git-repos.md](docs/private-git-repos.md) for information on setting up HAS and SPI for use with private git repositories.

### Deploying HAS


Once a secret has been created, simply run the following commands to deploy HAS:
```
make install
make deploy
```

### Specifying Alternate GitHub org

By default, HAS will use the `redhat-appstudio-appdata` org for the creation of GitOps repositories. If you wish to use your own account, or a different GitHub org, setting `GITHUB_ORG=<org>` before deploying will ensure that an alternate location is used.

For example:

`GITHUB_ORG=fake-organization make deploy` would deploy HAS configured to use github.com/fake-organization.

### Specifying Alternate Devfile Registry URL

By default, the production devfile registry URL will be used for `ComponentDetectionQuery`. If you wish to use a different devfile registry, setting `DEVFILE_REGISTRY_URL=<devfile registry url>`  before deploying will ensure that an alternate devfile registry is used.

For example:

`DEVFILE_REGISTRY_URL=https://myregistry make deploy` would deploy HAS configured to use https://myregistry.

### Disabling Webhooks for Local Dev

Webhooks require self-signed certificates to validate the resources. To disable webhooks during local dev and testing, export `ENABLE_WEBHOOKS=false`


Useful links:
* [HAS Project information page](https://docs.google.com/document/d/1axzNOhRBSkly3M2Y32Pxr1MBpBif2ljb-ufj0_aEt74/edit?usp=sharing)

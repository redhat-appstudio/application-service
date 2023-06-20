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

**Note:** In order for the controller tests to run, follow the instructions for [installing the Pact tools](#installing-pact-tools)

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

## Installing Pact Tools

The Pact tests in the controller package require pact tooling to be installed and on your path. Follow these instructions to do so:

1. Change directory to an appropriate folder (e.g. `/usr/local`)
2. Run `curl -fsSL https://raw.githubusercontent.com/pact-foundation/pact-ruby-standalone/master/install.sh | bash`
3. Add the pact tools' bin folder (e.g. `/usr/local/pact/bin`) to your path to your shell PATH. Ensure all binary files within the `bin/` folder has executable permissions
4. Run `go install github.com/pact-foundation/pact-go@v1` to install the `pact-go` tool
5. Run `pact-go install` to validate that all of the necessary Pact tools are installed

## Deploying the Operator

The following section outlines the steps to deploy HAS on a physical Kubernetes cluster.

### Setting up the AppStudio Build Service environment

* Install `OpenShift GitOps` from the in-cluster Operator Marketplace.
* `oc -n openshift-gitops apply -f https://raw.githubusercontent.com/redhat-appstudio/infra-deployments/main/argo-cd-apps/base/build.yaml`

As a user, upon creation of Component, Tekton resources would be created by the controller.

To use auto generated image repository for the Component's image add `image.redhat.com/generate: "true"` annotation to the Component.

If you wish to get 'working' PipelineRuns with user provided image repository, create an image pull secret and link it to the `pipeline` Service Account in the Component's namespace (in both `secrets` and `imagePullSecrets` sections).
See [Kubernetes docs](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials) for more information on how to create `Secrets` containing registry credentials.



### Creating a GitHub Secret for HAS

Before deploying the operator, you must ensure that a secret, `has-github-token`, exists in the namespace where HAS will be deployed. This secret must contain a key, `tokens`, whose value points to a comma separated list, without spaces, of key-value pairs of token names and tokens, delimited by a colon. 

For example, on OpenShift:

<img width="801" alt="Screenshot 2023-03-22 at 3 53 11 PM" src="https://user-images.githubusercontent.com/6880023/227020767-30b3db08-e191-4ec1-81df-81ae2df55d79.png">

Or via command-line:

```bash
application-service % kubectl create secret generic has-github-token --from-literal=tokens=token1:ghp_faketoken,token2:ghp_anothertoken,token3:ghp_thirdtoken
```

Any token that is used here must have the following permissions set:
- `repo`
- `delete_repo`

In addition to this, each GitHub token must be associated with an account that has write access to the GitHub organization you plan on using with HAS (see next section).


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

### Useful links:
* [HAS Project information page](https://docs.google.com/document/d/1axzNOhRBSkly3M2Y32Pxr1MBpBif2ljb-ufj0_aEt74/edit?usp=sharing)
* Every Prow job executed by the CI system generates an artifacts directory containing information about that execution and its results. This [document](https://docs.ci.openshift.org/docs/how-tos/artifacts/) describes the contents of this directory and how they can be used to investigate the steps by the job.
* For more information on the GitOps resource generation, please refer to the [gitops-generation](./docs/gitops-generation.md) documentation
* Contract testing using a Pact framework is part of unit tests. Follow [this documentation](pactTests.md) to learn more.

## Contributions

Please see our [CONTRIBUTING](./docs/CONTRIBUTING.md) for more information.

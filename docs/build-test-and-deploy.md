# Build, Test and Deploy

## Build
This operator provides a `Makefile` to run all the usual development tasks. If you simply run `make` without any arguments, you'll get a list of available "targets".

To build the operator binary run:

```
make build
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

## Test

To test the code:

```
make test
```

**Note:** In order for the controller tests to run, follow the instructions for [installing the Pact tools](./installing-pact-tools.md)

## Deploy

The following section outlines the steps to deploy application-service on a physical Kubernetes cluster.

### Deploying on a Local Cluster

#### Creating a GitHub Secret for application-service

Before deploying the operator, you must ensure that a secret `has-github-token`, exists in the namespace where application-service will be deployed. This secret must contain a key `tokens`, whose value points to a comma separated list without spaces of key-value pairs of token names and tokens, delimited by a colon. 

For example, on OpenShift:

<img width="801" alt="Screenshot 2023-03-22 at 3 53 11 PM" src="https://user-images.githubusercontent.com/6880023/227020767-30b3db08-e191-4ec1-81df-81ae2df55d79.png">

Or via command-line:

```bash
application-service % kubectl create secret generic has-github-token --from-literal=tokens=token1:ghp_faketoken,token2:ghp_anothertoken,token3:ghp_thirdtoken
```

Any token that is used here must have the following permissions set:
- `repo`
- `delete_repo`

In addition to this, each GitHub token must be associated with an account that has write access to the GitHub organization you plan on using with application-service.

#### Using Private Git Repos

The application-service component requires SPI to be set up in order to work with private git repositories.

Please refer to the [instructions](./private-git-repos.md) for information on setting up application-service and SPI for use with private git repositories.

#### Deploy application-service


Once a secret has been created, simply run the following commands to deploy application-service:
```
make install
make deploy
```

The application-service deployment can be further configured. Please refer to the sections below for your needs.

#### Specifying Alternate GitHub org

By default, application-service will use the `redhat-appstudio-appdata` org for the creation of GitOps repositories. If you wish to use your own account, or a different GitHub org, setting `GITHUB_ORG=<org>` before deploying will ensure that an alternate location is used.

For example:

`GITHUB_ORG=fake-organization make deploy` would deploy application-service configured to use github.com/fake-organization.

#### Specifying Alternate Devfile Registry URL

By default, the production devfile registry URL will be used for `ComponentDetectionQuery`. If you wish to use a different devfile registry, setting `DEVFILE_REGISTRY_URL=<devfile registry url>`  before deploying will ensure that an alternate devfile registry is used.

For example:

`DEVFILE_REGISTRY_URL=https://myregistry make deploy` would deploy application-service configured to use https://myregistry.

### Deploying Locally

#### Disabling Webhooks for Local Development

Webhooks require self-signed certificates to validate the Kubernetes resources. To disable webhooks during local development and testing, export `ENABLE_WEBHOOKS=false`

#### Setting the GitHub Token Environment variable

Either of the Environment variable `GITHUB_AUTH_TOKEN` or `GITHUB_TOKEN_LIST` needs to be set.

The `GITHUB_AUTH_TOKEN` variable is the legacy format and requires one token, example `GITHUB_AUTH_TOKEN=ghp_faketoken`. The `GITHUB_TOKEN_LIST` can take a list of tokens, example `GITHUB_TOKEN_LIST=token1:ghp_faketoken,token2:ghp_anothertoken`.

#### Executing the application-service binary

The application-service controller manager can be run locally on your development environment (example, laptop). For example, to build and run the executable manager:

```
make install
make build
./bin/manager
```

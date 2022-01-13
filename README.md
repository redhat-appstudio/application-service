# Hybrid Application Service (HAS)
An Kubernetes operator to create and manage applications and control the lifecycle of applications.


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

### Specifying Alternate GitHub org

By default, HAS will use the `redhat-appstudio-appdata` GitHub org for the creation of GitOps repositories. If you wish to use a different org, you can create a ConfigMap, `github-config`, in the same namespace as HAS before deploying, that references the org you wish to create GitOps repositories in. There should be a single key-value pair in the ConfigMap, where the key is `GITHUB_ORG`, and the value is the name of the GitHub organization you wish to use.

For example:

`kubectl create configmap github-config --from-literal=GITHUB_ORG=test-org` would create a ConfigMap that tells HAS to use the `test-org`.

**Note:** You must specify a GitHub organization if overriding the default value. GitHub accounts are not supported in lieu of organizations at this time.

### Creating a GitHub Secret for HAS

Before deploying the operator, you must ensure that a secret, `has-github-token`, exists in the namespace where HAS will be deployed. This secret must contain a key-value pair, where the key is `token` and where the value points to a valid GitHub Personal Access Token.

The token that is used here must have the following permissions set:
- `repo`
- `delete_repo`

In addition to this, the GitHub token must be associated with an account that has write access to the GitHub organization you plan on using with HAS (see above).

For example, on OpenShift:
<img width="862" alt="Screen Shot 2021-12-14 at 1 08 43 AM" src="https://user-images.githubusercontent.com/6880023/145942734-63422532-6fad-4017-9d26-79436fe241b8.png">

### Deploying HAS

Once the GitHub secret has been created, simply run the following commands to deploy HAS:
```
make install
make deploy
```

Useful links:
* [HAS Project information page](https://docs.google.com/document/d/1axzNOhRBSkly3M2Y32Pxr1MBpBif2ljb-ufj0_aEt74/edit?usp=sharing)

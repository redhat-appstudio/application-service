# Hybrid Application Service (HAS)

[![codecov](https://codecov.io/gh/redhat-appstudio/application-service/branch/main/graph/badge.svg)](https://codecov.io/gh/redhat-appstudio/application-service)

## Overview

A Kubernetes operator to create and manage applications and control the lifecycle of applications.

This repository is closely associated with the [application-api](https://github.com/redhat-appstudio/application-api/) repository, which contains the public APIs.

## Documentation

### ⚡ Project Info
* [HAS/application-service project information page](https://docs.google.com/document/d/1axzNOhRBSkly3M2Y32Pxr1MBpBif2ljb-ufj0_aEt74/edit?usp=sharing) - document detailing the project
* [Google Drive](https://drive.google.com/drive/u/0/folders/1pqESr0oc2ldtfj9RDx65vD_KdkgY_G9h) - Lot's of information for new people to the project

### 🔥 Developers

* [Building](./docs/build-and-test.md) - building and testing the application-service
* [Deployment](./docs/deploy.md) - deploying application-service
* [Serviceability](./docs/serviceability.md) - application-service serviceability like accessing and understanding logs, debugging, common problems and FAQs

### ⭐ Other Info

* [Gitops-generation](./docs/gitops-generation.md) - more information on the GitOps resource generation from application-service
* [Pact tests](./docs/pact-tests.md) - contract testing using a Pact framework is part of the unit tests
* [OpenShift CI job artifacts](https://docs.ci.openshift.org/docs/how-tos/artifacts/) - Prow job executed by the CI system generates an artifacts directory, this doc describes the contents of this directory and how they can be used to investigate the job steps

## Release

For more information on the application-service release policy, please read the release [guideline](./docs/release.md).

## Contributions

If you would like to contribute to application-service, please be so kind to read our [CONTRIBUTING](./docs/CONTRIBUTING.md) guide for more information.

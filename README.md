# Hybrid Application Service (HAS)

[![codecov](https://codecov.io/gh/redhat-appstudio/application-service/branch/main/graph/badge.svg)](https://codecov.io/gh/redhat-appstudio/application-service)

## Overview

A Kubernetes operator to create, manage and control the lifecycle of applications and components.

This repository is closely associated with the [application-api](https://github.com/konflux-ci/application-api/) repository, which contains the Kubernetes CRD definitions for the application-service specific resources - `Application`, `Component` and `ComponentDetectionQuery`.

## Documentation

### ⚡ Project Info
* [HAS/application-service project information page](https://docs.google.com/document/d/1axzNOhRBSkly3M2Y32Pxr1MBpBif2ljb-ufj0_aEt74/edit?usp=sharing) - document detailing the project
* [Google Drive](https://drive.google.com/drive/u/0/folders/1pqESr0oc2ldtfj9RDx65vD_KdkgY_G9h) - information for people new to the project

### 🔥 Developers

* [Build, Test and Deploy](./docs/build-test-and-deploy.md) - build, test and deploy the application-service
* [Serviceability](./docs/serviceability.md) - serviceability information like accessing and understanding logs, debugging, common problems and FAQs

### ⭐ Other Info

* [Gitops-generation](./docs/gitops-generation.md) - information on the GitOps resource generation from application-service
* [Pact tests](./docs/pact-tests.md) - contract tests using a Pact framework (part of the unit tests)
* [OpenShift CI job artifacts](https://docs.ci.openshift.org/docs/how-tos/artifacts/) - Prow job executed by the CI system generates an artifacts directory, this document describes the contents of this directory and how they can be used to investigate the job steps

## Release

For more information on the application-service release policy, please read the release [guideline](./docs/release.md).

## Contributions

If you would like to contribute to application-service, please be so kind to read our [CONTRIBUTING](./docs/CONTRIBUTING.md) guide for more information.

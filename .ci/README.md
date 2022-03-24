# Application Service CI documentation

Currently in application-service all tests are running in [Openshift CI](https://prow.ci.openshift.org/?job=*application*service*).

## Openshift CI

Openshift CI is a Kubernetes based CI/CD system. Jobs can be triggered by various types of events and report their status to many different services. In addition to job execution, Openshift CI provides GitHub automation in a form of policy enforcement, chat-ops via /foo style commands and automatic PR merging.

All documentation about how to onboard components in Openshift CI can be found in the Openshift CI jobs [repository](https://github.com/openshift/release). All application-service jobs configurations are defined in https://github.com/openshift/release/tree/master/ci-operator/config/redhat-appstudio/application-service.

- `has-e2e` Run has-suites suites from [e2e-tests](https://github.com/redhat-appstudio/e2e-tests/pkg/tests/has) repository.

The test container to run the e2e tests in Openshift Ci is builded from: https://github.com/redhat-appstudio/application-service/blob/main/.ci/openshift-ci/Dockerfile

The following environments are used to launch the CI tests in Openshift CI:

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `HAS_CONTROLLER_IMAGE` | no | An valid application service container without tag. | `quay.io/redhat-appstudio/application-service` |
| `HAS_CONTROLLER_IMAGE_TAG` | no | An valid application service container tag. | `next` |
| `GITHUB_TOKEN` | yes | A github token used to create AppStudio applications in GITHUB  | ''  |
| `QUAY_TOKEN` | yes | A quay token to push components images to quay.io | '' |

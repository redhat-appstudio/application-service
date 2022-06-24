#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

command -v e2e-appstudio >/dev/null 2>&1 || { echo "e2e-appstudio bin is not installed. Please install it from: https://github.com/redhat-appstudio/e2e-tests."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed. Aborting."; exit 1; }

export HAS_PR_OWNER HAS_PR_SHA
export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));
export TEST_SUITE="has-suite"
export APPLICATION_NAMESPACE="openshift-gitops"
export APPLICATION_NAME="all-components-staging"

# HAS_CONTROLLER_IMAGE it is application-service controller image builded in openshift CI job workflow. More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
# Container env defined at: https://github.com/openshift/release/blob/master/ci-operator/config/redhat-appstudio/application-service/redhat-appstudio-application-service-main.yaml#L6-L7
# Openshift CI generate the application service container value as registry.build01.ci.openshift.org/ci-op-83gwcnmk/pipeline@sha256:8812e26b50b262d0cc45da7912970a205add4bd4e4ff3fed421baf3120027206. Need to get the image without sha.
export OPENSHIFT_CI_CONTROLLER_IMAGE=${HAS_CONTROLLER_IMAGE%@*}
# Tag defined at: https://github.com/openshift/release/blob/master/ci-operator/config/redhat-appstudio/application-service/redhat-appstudio-application-service-main.yaml#L8
export OPENSHIFT_CI_CONTROLLER_TAG=${HAS_CONTROLLER_IMAGE_TAG:-"redhat-appstudio-has-image"}

export HAS_IMAGE_REPO=${OPENSHIFT_CI_CONTROLLER_IMAGE:-"quay.io/redhat-appstudio/application-service"}
export HAS_IMAGE_TAG=${OPENSHIFT_CI_CONTROLLER_TAG:-"next"}

if [[ -n "${JOB_SPEC}" && "${REPO_NAME}" == "application-service" ]]; then
    # Extract PR author and commit SHA to also override default kustomization in infra-deployments repo
    # https://github.com/redhat-appstudio/infra-deployments/blob/1d623e2278aecbdf266f374e02cf3f55de62a42f/hack/preview.sh#L91
    HAS_PR_OWNER=$(jq -r '.refs.pulls[0].author' <<< "$JOB_SPEC")
    HAS_PR_SHA=$(jq -r '.refs.pulls[0].sha' <<< "$JOB_SPEC")
fi

# Available openshift ci environments https://docs.ci.openshift.org/docs/architecture/step-registry/#available-environment-variables
export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/appstudio"}

function waitHASApplicationToBeReady() {
    while [ "$(kubectl get applications.argoproj.io has -n openshift-gitops -o jsonpath='{.status.health.status}')" != "Healthy" ]; do
        sleep 30s
        echo "[INFO] Waiting for HAS to be ready."
    done
}

function waitAppStudioToBeReady() {
    while [ "$(kubectl get applications.argoproj.io ${APPLICATION_NAME} -n ${APPLICATION_NAMESPACE} -o jsonpath='{.status.health.status}')" != "Healthy" ] ||
          [ "$(kubectl get applications.argoproj.io ${APPLICATION_NAME} -n ${APPLICATION_NAMESPACE} -o jsonpath='{.status.sync.status}')" != "Synced" ]; do
        sleep 1m
        echo "[INFO] Waiting for AppStudio to be ready."
    done
}

function waitBuildToBeReady() {
    while [ "$(kubectl get applications.argoproj.io build -n ${APPLICATION_NAMESPACE} -o jsonpath='{.status.health.status}')" != "Healthy" ] ||
          [ "$(kubectl get applications.argoproj.io build -n ${APPLICATION_NAMESPACE} -o jsonpath='{.status.sync.status}')" != "Synced" ]; do
        sleep 1m
        echo "[INFO] Waiting for Build to be ready."
    done
}

function executeE2ETests() {
    # E2E instructions can be found: https://github.com/redhat-appstudio/e2e-tests
    # The e2e binary is included in Openshift CI test container from the dockerfile: https://github.com/redhat-appstudio/infra-deployments/blob/main/.ci/openshift-ci/Dockerfile
    curl https://raw.githubusercontent.com/redhat-appstudio/e2e-tests/main/scripts/e2e-openshift-ci.sh | bash -s

    # The bin will be installed in tmp folder after executing e2e-openshift-ci.sh script
    cd "${WORKSPACE}/tmp/e2e-tests"
    ./bin/e2e-appstudio --ginkgo.junit-report="${ARTIFACT_DIR}"/e2e-report.xml --ginkgo.focus="${TEST_SUITE}" --ginkgo.progress --ginkgo.v --ginkgo.no-color
}


# Initiate openshift ci users
export KUBECONFIG_TEST="/tmp/kubeconfig"
curl https://raw.githubusercontent.com/redhat-appstudio/e2e-tests/main/scripts/provision-openshift-user.sh | bash -s
export KUBECONFIG="${KUBECONFIG_TEST}"

curl https://raw.githubusercontent.com/redhat-appstudio/e2e-tests/main/scripts/install-appstudio-e2e-mode.sh | bash -s install

export -f waitAppStudioToBeReady
export -f waitBuildToBeReady
export -f waitHASApplicationToBeReady

# Install AppStudio Controllers and wait for HAS and other AppStudio application to be running.
timeout --foreground 10m bash -c waitAppStudioToBeReady
timeout --foreground 10m bash -c waitBuildToBeReady
timeout --foreground 10m bash -c waitHASApplicationToBeReady

executeE2ETests

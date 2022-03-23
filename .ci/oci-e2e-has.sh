#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

command -v e2e-appstudio >/dev/null 2>&1 || { echo "e2e-appstudio bin is not installed. Please install it from: https://github.com/redhat-appstudio/e2e-tests."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed. Aborting."; exit 1; }

export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));
export TEST_SUITE="has-suite"
export APPLICATION_NAMESPACE="openshift-gitops"
export APPLICATION_NAME="all-components-staging"

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
    e2e-appstudio --ginkgo.junit-report="${ARTIFACTS_DIR}"/e2e-report.xml --ginkgo.focus="has-suite"
}

curl https://raw.githubusercontent.com/redhat-appstudio/e2e-tests/main/scripts/install-appstudio-e2e-mode.sh | bash -s install

export -f waitAppStudioToBeReady
export -f waitBuildToBeReady
export -f waitHASApplicationToBeReady

# Install AppStudio Controllers and wait for HAS and other AppStudio application to be running.
timeout --foreground 10m bash -c waitAppStudioToBeReady
timeout --foreground 10m bash -c waitBuildToBeReady
timeout --foreground 10m bash -c waitHASApplicationToBeReady

executeE2ETests

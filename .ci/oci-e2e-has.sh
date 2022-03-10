# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u
export HAS_CONTROLLER_IMAGE=quay.io/redhat-appstudio/application-service:next

command -v e2e-appstudio >/dev/null 2>&1 || { echo "e2e-appstudio bin is not installed. Please install it from: https://github.com/redhat-appstudio/e2e-tests."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed. Aborting."; exit 1; }

if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo - e "[ERROR] GITHUB_TOKEN env is not set. Aborting."
fi

if [[ -z "${QUAY_TOKEN}" ]]; then
  echo - e "[ERROR] QUAY_TOKEN env is not set. Aborting."
fi

if [[ -z "${HAS_CONTROLLER_IMAGE}" ]]; then
  echo - e "[ERROR] HAS_CONTROLLER_IMAGE env is not set. Aborting."
fi

export TEST_BRANCH_ID=$(date +%s)
export MY_GIT_FORK_REMOTE="qe"
export MY_GITHUB_ORG="redhat-appstudio-qe"
export MY_GITHUB_TOKEN="${GITHUB_TOKEN}"
export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));

# Available openshift ci environments https://docs.ci.openshift.org/docs/architecture/step-registry/#available-environment-variables
export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/appstudio"}

# Secrets used by pipelines to push component containers to quay.io
function createControllerSecrets() {
    echo -e "[INFO] Creating has github token secret"
    kubectl create namespace application-service || true
    kubectl create secret generic has-github-token -n application-service --from-literal token=$GITHUB_TOKEN || true

    echo "$QUAY_TOKEN" | base64 --decode > docker.config
    kubectl create secret docker-registry redhat-appstudio-registry-pull-secret -n  application-service --from-file=.dockerconfigjson=docker.config
    kubectl create secret docker-registry redhat-appstudio-staginguser-pull-secret -n  application-service --from-file=.dockerconfigjson=docker.config
    rm docker.config
}

function waitHASApplicationToBeReady() {
    while [ "$(kubectl get applications.argoproj.io has -n openshift-gitops -o jsonpath='{.status.health.status}')" != "Healthy" ]; do
        sleep 3m
        echo "[INFO] Waiting for HAS to be ready."
    done
}

function cloneInfraDeployments() {
    git clone https://github.com/redhat-appstudio/infra-deployments.git ./tmp/infra-deployments
}

function executeE2ETests() {
    # E2E instructions can be found: https://github.com/redhat-appstudio/e2e-tests
    # The e2e binary is included in Openshift CI test container from the dockerfile: https://github.com/redhat-appstudio/infra-deployments/blob/main/.ci/openshift-ci/Dockerfile
    e2e-appstudio --ginkgo.junit-report="${ARTIFACTS_DIR}"/e2e-report.xml --ginkgo.focus="has-suite"
}

function addQERemote() {
    cd "$WORKSPACE"/tmp/infra-deployments
    git remote add ${MY_GIT_FORK_REMOTE} https://github.com/redhat-appstudio-qe/infra-deployments.git
    /bin/bash hack/bootstrap-cluster.sh preview

    cd "$WORKSPACE"
}

# Replace default image with an image builded from PR code
function changeContainerImage() {
  HAS_CONTAINER_NAME=${HAS_CONTROLLER_IMAGE%@*}
  yq -M eval ".images[0].newName |= \"$HAS_CONTAINER_NAME\"" -i "$WORKSPACE"/tmp/infra-deployments/components/has/kustomization.yaml
  # The tag is constant redhat-appstudio-has-image. See: https://github.com/openshift/release/blob/master/ci-operator/config/redhat-appstudio/application-service/redhat-appstudio-application-service-main.yaml#L8
  yq -M eval ".images[0].newTag |= \"redhat-appstudio-has-image\"" -i "$WORKSPACE"/tmp/infra-deployments/components/has/kustomization.yaml
}

createControllerSecrets
cloneInfraDeployments
addQERemote
changeContainerImage

export -f waitHASApplicationToBeReady
# Install AppStudio Controllers and wait for HAS to be installed
timeout --foreground 10m bash -c waitHASApplicationToBeReady

# Just a sleep before starting the tests
sleep 2m
executeE2ETests

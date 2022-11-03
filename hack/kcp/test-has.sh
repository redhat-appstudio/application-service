#!/bin/bash
set -e

HAS_IMAGE=$1

# Test path needs to point to a valid HAS repo folder
TESTPATH=$2

KCP_KUBECONFIG=$TESTPATH/.kcp/admin.kubeconfig

function waitForKCPToBeReady() {
    while true; do
        KUBECONFIG=$PWD/.kcp/admin.kubeconfig kubectl kcp ws
        if [[ $? -eq 0 ]]; then
            break
        fi
        echo "[INFO] Waiting for KCP to be ready."
        sleep 5
    done
}

function waitForSyncTargetToBeReady() {
    while [ "$(kubectl api-resources --kubeconfig $PWD/.kcp/admin.kubeconfig)" ]; do
        echo "[INFO] Waiting for KCP to be ready."
        sleep 5
    done
}

function setupTests() {
    # Create a workspace for HAS
    echo "[INFO] Creating test workspace on KCP"
    KUBECONFIG=$KCP_KUBECONFIG kubectl kcp ws
    KUBECONFIG=$KCP_KUBECONFIG kubectl kcp ws create tests --enter

    # Generate the syncer.yaml
    KUBECONFIG=$KCP_KUBECONFIG kubectl kcp workload sync minikube \
    --syncer-image ghcr.io/kcp-dev/kcp/syncer:v0.9.1 \
    --output-file=syncer.yaml

    # On Minikube, create the syncer resources
    kubectl create -f syncer.yaml

    # Wait for the SyncTarget to become ready
    KUBECONFIG=$KCP_KUBECONFIG kubectl wait synctargets minikube --for condition=Ready --timeout=120s

    # Create namespace and stub github secret for HAS
    KUBECONFIG=$KCP_KUBECONFIG kubectl create ns application-service-system
    KUBECONFIG=$KCP_KUBECONFIG kubectl create secret generic has-github-token --from-literal=token=testvalue -n application-service-system
}

# Execute tests deploys HAS on KCP, validates it becomes ready, and that a CDQ resource successfully completes
function executeTests() {
    # Deploy HAS
    echo "[INFO] Deploying HAS on KCP"
    KUBECONFIG=$KCP_KUBECONFIG IMG=$HAS_IMAGE make deploy-kcp

    # Wait for HAS to become ready
    echo "[INFO] Waiting for HAS deployment rollout to succeed"
    KUBECONFIG=$KCP_KUBECONFIG kubectl rollout status deployment application-service-controller-manager -n application-service-system --timeout=300s

    # Create a CDQ and validate it succeeds on KCP
    echo "[INFO] Creating a ComponentDetectionResource on KCP"
    KUBECONFIG=$KCP_KUBECONFIG kubectl create -f $TESTPATH/config/samples/componentdetectionquery/componentdetectionquery-basic.yaml
    KUBECONFIG=$KCP_KUBECONFIG kubectl wait hcdq componentdetectionquery-sample --for condition=Completed --timeout=120s
}

# Start KCP
kcp start > output.log 2>&1 &

# Wait for KCP to become available

export -f waitForKCPToBeReady

timeout --foreground 1m bash -c waitForKCPToBeReady

setupTests

executeTests
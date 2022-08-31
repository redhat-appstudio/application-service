#!/bin/bash

#trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT

## This script runs the HAS controller tests against a KCP environment
CURDIR=`pwd`
rm -rf $CURDIR/.kcp || true
rm -rf $CURDIR/kcp-output.log || true
export KUBECONFIG=$CURDIR/.kcp/admin.KUBECONFIG

# Start KCP
kcp start &> kcp-output.log &

# Wait for KCP to finish starting
timeout 2m bash -c 'until cat kcp-output.log | grep "finished bootstrapping root workspace"; do echo Retry; sleep 5; done'
if [[ $? -ne 0 ]]; then
   echo "KCP failed to become ready"
   cat kcp-output.log
   exit 1
fi

# Run the controller tests
# Set USE_EXISTING_CLUSTER to tell envtest to use KCPs kubeconfig
export USE_EXISTING_CLUSTER=true
go test ./controllers -coverprofile cover.out -v
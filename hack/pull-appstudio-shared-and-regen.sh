#!/bin/bash

# Simple script to pull the Appstudio Shared CRDs and regenerate the KCP API resources
hackfolder="$(realpath $(dirname ${BASH_SOURCE[0]}))"
curl -o $hackfolder/../config/crd/bases/appstudio-shared-customresourcedefinitions.yaml https://raw.githubusercontent.com/redhat-appstudio/managed-gitops/main/appstudio-shared/manifests/appstudio-shared-customresourcedefinitions.yaml

$hackfolder/generate-kcp-api.sh
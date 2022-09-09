#!/bin/bash

# This script copies the config directory containing the kustomize templates into a subdirectory under .tmp.

set -eux

# the path to the kustomize executable
KUSTOMIZE=$1

echo "********"
echo $KUSTOMIZE
# the name of the deployment - only used as a part of the directory the templates are copied into
DEPL_NAME=$2

# The name of the kustomize overlay directory to apply
OVERLAY=$3

THIS_DIR="$(dirname "$(realpath "$0")")"
TEMP_DIR="${THIS_DIR}/../.tmp/deployment_${DEPL_NAME}"

OVERLAY_DIR="${TEMP_DIR}/${OVERLAY}"

# we need this to keep kustomize patches intact
export patch="\$patch"

mkdir -p "${TEMP_DIR}"
cp -r "${THIS_DIR}/../config/"* "${TEMP_DIR}"
find "${TEMP_DIR}" -name '*.yaml' | while read -r f; do
  tmp=$(mktemp)
  envsubst > "$tmp" < "$f"
  mv "$tmp" "$f"
done

CURDIR=$(pwd)

cd "${OVERLAY_DIR}" || exit

GITHUB_ORG=${GITHUB_ORG} DEVFILE_REGISTRY_URL=${DEVFILE_REGISTRY_URL} ${KUSTOMIZE} build . | kubectl apply -f -

cd "${CURDIR}" || exit

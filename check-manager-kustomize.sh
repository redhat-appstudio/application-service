#!/bin/sh

THIS_DIR="$(dirname "$(realpath "$0")")"
MANAGER_KUSTOMIZATION="$( realpath ${THIS_DIR}/config/manager/kustomization.yaml)"

cat ${MANAGER_KUSTOMIZATION} | grep "newName: quay.io/konflux-ci/application-service"

exit $?
#!/bin/bash
# exit immediately when a command fails
set -ex
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables

if [[ -n ${CLONEREFS_OPTIONS} ]]; then
    AUTHOR_LINK=$(jq -r '.refs[0].pulls[0].author_link' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    GITHUB_ORGANIZATION=$(jq -r '.refs[0].org' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    GITHUB_HEAD_REPO=$(jq -r '.refs[0].repo' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')

    curl https://api.github.com/repos/"${GITHUB_ORGANIZATION}"/"${GITHUB_HEAD_REPO}"/pulls/"${PULL_NUMBER}" | jq --raw-output .head.ref
fi

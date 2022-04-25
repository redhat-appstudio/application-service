#!/bin/bash
# exit immediately when a command fails
set -ex
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables

if [[ -n ${CLONEREFS_OPTIONS} ]]; then
    AUTHOR=$(jq -r '.refs[0].pulls[0].author' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    GITHUB_ORGANIZATION=$(jq -r '.refs[0].org' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    GITHUB_REPO=$(jq -r '.refs[0].repo' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')

    PR_BRANCH_REF=$(curl https://api.github.com/repos/"${GITHUB_ORGANIZATION}"/"${GITHUB_REPO}"/pulls/"${PULL_NUMBER}" | jq --raw-output .head.ref)
    curl https://api.github.com/repos/"${AUTHOR}"/e2e-tests/branches | jq '.[] | select(.name=="'${PR_BRANCH_REF}'")'
fi


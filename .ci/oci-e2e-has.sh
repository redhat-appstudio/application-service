#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}
export E2E_CLONE_BRANCH="main"
export E2E_REPO_LINK="https://github.com/redhat-appstudio/e2e-tests.git"

mkdir -p tmp/

if [[ -n ${CLONEREFS_OPTIONS} ]]; then
    AUTHOR=$(jq -r '.refs[0].pulls[0].author' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    AUTHOR_LINK=$(jq -r '.refs[0].pulls[0].author_link' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    GITHUB_ORGANIZATION=$(jq -r '.refs[0].org' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
    GITHUB_REPO=$(jq -r '.refs[0].repo' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')

    PR_BRANCH_REF=$(curl https://api.github.com/repos/"${GITHUB_ORGANIZATION}"/"${GITHUB_REPO}"/pulls/"${PULL_NUMBER}" | jq --raw-output .head.ref)
    AUTHOR_E2E_BRANCH=$(curl https://api.github.com/repos/"${AUTHOR}"/e2e-tests/branches | jq '.[] | select(.name=="'${PR_BRANCH_REF}'")')

    if [ -z "${AUTHOR_E2E_BRANCH}" ]; then
        echo "[INFO] ${PR_BRANCH_REF} not exists in ${AUTHOR_LINK}/e2e-tests. Using ${E2E_CLONE_BRANCH} to clone the e2e-tests"
    else
        echo "[INFO] Cloning e2e-tests from branch ${PR_BRANCH_REF} repository ${AUTHOR_LINK}/e2e-tests"
        E2E_CLONE_BRANCH=${PR_BRANCH_REF}
        E2E_REPO_LINK="${AUTHOR_LINK}/e2e-tests.git"
    fi
fi

git clone -b "${E2E_CLONE_BRANCH}" "${E2E_REPO_LINK}" "$WORKSPACE"/tmp/e2e-tests

cd "$WORKSPACE"/tmp/e2e-tests
make build
chmod 755 "$WORKSPACE"/tmp/e2e-tests/bin/e2e-appstudio
export PATH="$WORKSPACE"/tmp/e2e-tests/bin:${PATH}

e2e-appstudio --help

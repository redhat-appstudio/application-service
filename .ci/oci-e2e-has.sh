#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

#

printenv




        # get branch ref of the fork the PR was created from
        echo ${CLONEREFS_OPTIONS}
        AUTHOR_LINK=$(jq -r '.refs[0].pulls[0].author_link' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
        PULL_PULL_SHA=${PULL_PULL_SHA:-$(jq -r '.refs[0].pulls[0].sha' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')}
        echo "using author link ${AUTHOR_LINK}"

        REPO_URL=${AUTHOR_LINK}/e2e-tests
        echo "branches of ${REPO_URL} - trying to detect the branch name we should use for pairing."
        curl ${REPO_URL}.git/info/refs?service=git-upload-pack --output -
        GET_BRANCH_NAME=$(curl ${REPO_URL}.git/info/refs?service=git-upload-pack --output - 2>/dev/null | grep -a ${PULL_PULL_SHA} || true)
            
        BRANCH_REF=$(echo ${GET_BRANCH_NAME} | awk '{print $2}')
        echo "detected branch ref ${BRANCH_REF}"
        # retrieve the branch name
        BRANCH_NAME=$(echo ${BRANCH_REF} | awk -F'/' '{print $3}')
        echo -e "AAAA"
        echo $BRANCH_NAME
        echo "######"
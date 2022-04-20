#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

#
REPOSITORY_NAME="e2e-tests"
printenv
    if [[ -n "${CI}${CLONEREFS_OPTIONS}" ]]; then
        if [[ -n ${CLONEREFS_OPTIONS} ]]; then
            # get branch ref of the fork the PR was created from
            AUTHOR_LINK=$(jq -r '.refs[0].pulls[0].author_link' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
            PULL_PULL_SHA=${PULL_PULL_SHA:-$(jq -r '.refs[0].pulls[0].sha' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')}
            echo "using author link ${AUTHOR_LINK}"
            echo "using pull sha ${PULL_PULL_SHA}"
            # get branch ref of the fork the PR was created from
            REPO_URL=${AUTHOR_LINK}/e2e-tests
            echo "branches of ${REPO_URL} - trying to detect the branch name we should use for pairing."
            curl ${REPO_URL}.git/info/refs?service=git-upload-pack --output -
            GET_BRANCH_NAME=$(curl ${REPO_URL}.git/info/refs?service=git-upload-pack --output - 2>/dev/null | grep -a ${PULL_PULL_SHA} || true)
            if [[ $(echo ${GET_BRANCH_NAME} | wc -l) > 1 ]]; then \
                echo "###################################  ERROR DURING THE E2E TEST SETUP  ###################################
There were found more branches with the same latest commit '${PULL_PULL_SHA}' in the repo ${REPO_URL} - see:
${GET_BRANCH_NAME}
It's not possible to detect the correct branch this PR is made for.
Please delete the unrelated branch from your fork and rerun the e2e tests.
Note: If you have already deleted the unrelated branch from your fork, it can take a few hours before the
      github api is updated so the e2e tests may still fail with the same error until then.
##########################################################################################################"
                exit 1
            fi
            BRANCH_REF=$(echo ${GET_BRANCH_NAME} | awk '{print $2}')
            echo "detected branch ref ${BRANCH_REF}"
            # retrieve the branch name
            BRANCH_NAME=$(echo ${BRANCH_REF} | awk -F'/' '{print $3}')
        else
            AUTHOR_LINK=https://github.com/${AUTHOR}
            BRANCH_REF=refs/heads/${GITHUB_HEAD_REF}
            BRANCH_NAME=${GITHUB_HEAD_REF}
            REPO_URL=${AUTHOR_LINK}/toolchain-e2e
        fi

        if [[ -n "${BRANCH_REF}" ]]; then \
            # check if a branch with the same ref exists in the user's fork of ${REPOSITORY_NAME} repo
            echo "branches of ${AUTHOR_LINK}/${REPOSITORY_NAME} - checking if there is a branch ${BRANCH_REF} we could pair with."
            curl ${AUTHOR_LINK}/${REPOSITORY_NAME}.git/info/refs?service=git-upload-pack --output -
            REMOTE_E2E_BRANCH=$(curl ${AUTHOR_LINK}/${REPOSITORY_NAME}.git/info/refs?service=git-upload-pack --output - 2>/dev/null | grep -a "${BRANCH_REF}$" | awk '{print $2}')
            echo "branch ref of the user's fork: \"${REMOTE_E2E_BRANCH}\" - if empty then not found"
            # check if the branch with the same name exists, if so then merge it with master and use the merge branch, if not then use master \
            if [[ -n "${REMOTE_E2E_BRANCH}" ]]; then \
                if [[ -f ${WAS_ALREADY_PAIRED_FILE} ]]; then \
                    echo "####################################  ERROR WHILE TRYING TO PAIR PRs  ####################################
There was an error while trying to pair this e2e PR with ${AUTHOR_LINK}/${REPOSITORY_NAME}@${BRANCH_REF}
The reason is that there was already detected a branch from another repo this PR could be paired with - see:
$(cat ${WAS_ALREADY_PAIRED_FILE})
It's not possible to pair a PR with multiple branches from other repositories.
Please delete one of the branches from your fork and rerun the e2e tests
Note: If you have already deleted one of the branches from your fork, it can take a few hours before the
      github api is updated so the e2e tests may still fail with the same error until then.
##########################################################################################################"
                    exit 1
                fi

                git config --global user.email "devtools@redhat.com"
                git config --global user.name "Devtools"

                echo -e "repository: ${AUTHOR_LINK}/${REPOSITORY_NAME} \nbranch: ${BRANCH_NAME}" > ${WAS_ALREADY_PAIRED_FILE}
                # add the user's fork as remote repo
                git --git-dir=${REPOSITORY_PATH}/.git --work-tree=${REPOSITORY_PATH} remote add external ${AUTHOR_LINK}/${REPOSITORY_NAME}.git
                # fetch the branch
                git --git-dir=${REPOSITORY_PATH}/.git --work-tree=${REPOSITORY_PATH} fetch external ${BRANCH_REF}

                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!    WARNING    !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                echo ""
                echo "The following command will try to merge the paired PR using fast-forward way."
                echo "If the command fails, then it means that the paired PR https://github.com/codeready-toolchain/${REPOSITORY_NAME}/ from branch ${BRANCH_NAME}"
                echo "is not up-to-date with master and the fast-forward merge cannot be performed."
                echo "If this happens, then rebase the PR with the latest changes from master and rerun this GH Actions build (or comment /retest in this PR)."
                echo "       https://github.com/codeready-toolchain/${REPOSITORY_NAME}/pulls?q=head%3A${BRANCH_NAME}"
                echo ""
                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                # merge the branch with master using fast-forward
                #git --git-dir=${REPOSITORY_PATH}/.git --work-tree=${REPOSITORY_PATH} merge --ff-only FETCH_HEAD
                # print information about the last three commits, so we know what was merged plus some additional context/history
                #git --git-dir=${REPOSITORY_PATH}/.git --work-tree=${REPOSITORY_PATH} log --ancestry-path HEAD~3..HEAD
                
                PAIRED=true
            fi
        fi
    fi

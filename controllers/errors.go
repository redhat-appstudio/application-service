package controllers

import (
	"fmt"

	"github.com/redhat-developer/gitops-generator/pkg/util"
)

type GitOpsParseRepoError struct {
	remoteURL string
	err       error
}

func (e *GitOpsParseRepoError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("unable to parse gitops repository %s due to error: %v", e.remoteURL, e.err)).Error()
}

type GitOpsCommitIdError struct {
	err error
}

func (e *GitOpsCommitIdError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("unable to retrieve gitops repository commit id due to error: %v", e.err)).Error()
}

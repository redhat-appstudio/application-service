#!/bin/sh
set -eux

# Invoke the GitOps generator for the specified option
if [ "$OPERATION" = "generate-base" ]; then
    ./gitops-generator --operation "$OPERATION" --namespace "$NAMESPACE" --repoURL "$REPOURL" --component "$RESOURCE" --branch "$BRANCH" --path "$CONTEXT"
else
    ./gitops-generator --operation "$OPERATION" --namespace "$NAMESPACE" --seb "$RESOURCE"
fi
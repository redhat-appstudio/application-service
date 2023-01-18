#!/bin/sh

# Invoke the GitOps generator for the specified option
if [ $OPERAION -eq "generate-base" ]; then
    ./gitops-generator --operation $OPERATION --namespace $NAMESPACE --repoURL $REPOURL --component $RESOURCE --branch $BRANCH --context $CONTEXT
else
    ./gitops-generator --operation $OPERATION --namespace $NAMESPACE --repoURL $REPOURL --seb $RESOURCE 
fi
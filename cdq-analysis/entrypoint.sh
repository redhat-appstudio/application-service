#!/bin/sh
set -eux

./cdq-analysis --name "$NAME" --namespace "$NAMESPACE" -- contextPath "$CONTEXT_PATH" \
            --revision "$REVISION" --URL "$URL" --DevfileRegistryURL "$DEVFILE_REGISTRY_URL" \
            --devfilePath "$DEVFILE_PATH" --dockerfilePath "$DOCKERFILE_PATH" --isDevfilePresent $IS_DEVFILE_PRESENT \
            --isDockerfilePresent $IS_DOCKERFILE_PRESENT --createK8sJob $CREATE_K8S_Job

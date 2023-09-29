#!/bin/sh
set -eux

./cdq-analysis --name "$NAME" --namespace "$NAMESPACE" --contextPath "$CONTEXT_PATH" \
            --revision "$REVISION" --URL "$URL" --devfileRegistryURL "$DEVFILE_REGISTRY_URL" \
            --createK8sJob $CREATE_K8S_Job

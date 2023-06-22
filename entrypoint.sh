#!/bin/sh
set -eux

if [[ "$ENVIRONMENT" == "staging" ]]; then
  echo "running tini"
  exec tini -- /manager $*
else
  echo "running manager"
  exec /manager $*
fi

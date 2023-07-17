#!/bin/sh
set -eux

echo "running manager under tini"
exec tini -- /manager $*


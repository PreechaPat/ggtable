#!/bin/bash

set -euo pipefail
# Get the short 10-character git commit hash
SHA_SHORT=$(git rev-parse --short=10 HEAD)
SHA_FULL=$(git rev-parse HEAD)

echo "Building ggtable image with revision ${SHA_FULL}"
docker build . -t ggtable:sha-${SHA_SHORT} -t "ggtable:latest" --no-cache --build-arg "VERSION=${SHA_FULL}" "$@"

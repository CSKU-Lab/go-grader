#!/bin/sh

echo "⌛ Starting isolate-test..."

# cleanup
docker stop isolate-test || true

docker build -t isolate-test -f docker/isolate-test/Dockerfile .
docker run --rm --privileged \
    --name isolate-test \
    isolate-test

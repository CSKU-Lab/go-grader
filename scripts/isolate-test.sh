#!/bin/sh

echo "⌛ Starting isolate-test..."

docker build -t isolate-test -f docker/isolate-test/Dockerfile .
docker run --rm --privileged \
    isolate-test

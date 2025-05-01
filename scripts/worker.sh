#!/bin/bash

echo "⌛ Starting worker..."
docker build -t worker -f docker/worker/Dockerfile .
docker run --rm --privileged \
    --name worker \
    --network host \
    worker

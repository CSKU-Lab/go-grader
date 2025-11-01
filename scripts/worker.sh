#!/bin/bash

echo "⌛ Starting worker..."
docker build -t worker -f docker/worker/Dockerfile .
docker run --rm --privileged \
    --network host \
    --env-file .env \
    --env ENV=docker \
    worker

#!/bin/bash

echo "⌛ Starting box..."

docker run --rm --privileged \
--name box \
-idt \
-v ./isolate-docker/config:/usr/local/etc/isolate \
sornchaithedev/all-isolate \
tail -f /dev/null

#!/bin/bash

echo "⌛ Starting box..."

docker run --rm --privileged \
--name box \
-idt \
--cgroupns=host \
-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
-v ./isolate-docker/config:/usr/local/etc/isolate \
cskulab/isolate-with-compilers \
tail -f /dev/null

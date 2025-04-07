#!/bin/bash

# Docker Compose wrapper script
# Usage: ./dc.sh <args>
# Example: ./dc.sh up
# Example: ./dc.sh down
# Example: ./dc.sh ps

# Check if the script is being run directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    # Pass all arguments to docker compose with the specified compose file
    docker compose -f docker/docker-compose.yaml "$@"
fi

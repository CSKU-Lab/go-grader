
worker:
	@echo "⌛ Starting worker..."
	@docker run --rm --privileged -v ./isolate-docker/config:/usr/local/etc/isolate worker

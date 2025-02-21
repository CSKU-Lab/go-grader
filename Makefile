
worker:
	@echo "⌛ Starting worker..."
	@docker build -t worker -f docker/worker/Dockerfile .
	@docker run --rm --privileged -v ./isolate-docker/config:/usr/local/etc/isolate worker

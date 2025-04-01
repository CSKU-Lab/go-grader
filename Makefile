
worker:
	@echo "⌛ Starting worker..."
	@docker build -t worker -f docker/worker/Dockerfile .
	@docker run --rm --privileged \
		-v ./isolate-docker/config:/usr/local/etc/isolate \
		-v ./configs/languages.json:/usr/local/etc/worker/languages.json \
		--network host \
		worker

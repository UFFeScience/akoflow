bash:
	@container_id=$$(docker ps --format "{{.ID}} {{.Image}}" | grep "vsc-akoflow" | awk '{print $$1}' | head -n 1) && \
	if [ -n "$$container_id" ]; then \
		echo "Using container ID: $$container_id"; \
		docker exec -it $$container_id bash; \
	else \
		echo "Error: Developer Container not found. Please start the container first."; \
	fi;
sh: bash
shell: bash
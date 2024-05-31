dev-deploy-server:
	echo "Deploying to dev server"
	echo "Build Server Image"
	docker buildx build -t ovvesley/akoflow-server:latest -f server.Dockerfile . --push --platform linux/amd64
	kubectl apply -f pkg/server/resource/akoflow.yaml

dev-log:
	kubectl logs $$(kubectl get pods | grep akoflow-server-deployment |grep Running |  awk '{print $$1}')

build-amd64:
	docker buildx build -t ovvesley/akoflow-server:latest -f server.Dockerfile . --platform=linux/amd64 --push

build-arm64:
	docker buildx build -t ovvesley/akoflow-server:latest -f server.Dockerfile . --platform=linux/arm64 --push

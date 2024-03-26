dev-deploy-server:
	echo "Deploying to dev server"
	echo "Build Server Image"
	docker buildx build -t ovvesley/uff-tcc-scientific-workflow-k8s:server -f server.Dockerfile . --push --platform linux/amd64
	kubectl apply -f pkg/server/resource/scik8sflow.yaml

dev-log:
	kubectl logs $$(kubectl get pods | grep scik8sflow-server-deployment |grep Running |  awk '{print $$1}')


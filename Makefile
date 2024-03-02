dev-deploy-server:
	echo "Deploying to dev server"
	echo "Build Server Image"
	docker buildx build -t ovvesley/uff-tcc-scientific-workflow-k8s:server -f server.Dockerfile .

	echo "Delete Server Deployment"
	@kubectl delete -f pkg/server/resource/cluster_manager.yaml || true
	kubectl apply -f pkg/server/resource/cluster_manager.yaml

dev-log:
	kubectl logs $$(kubectl get pods | grep cluster-manager-deployment |grep Running |  awk '{print $$1}')


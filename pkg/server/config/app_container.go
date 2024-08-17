package config

import (
	"github.com/ovvesley/akoflow/pkg/server/config/http_helper"
	"github.com/ovvesley/akoflow/pkg/server/config/http_render_view"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"net/http"
)

const DEFAULT_NAMESPACE = "akoflow"

type AppContainer struct {
	Repository       appContainerRepository
	Connector        appContainerConnector
	DefaultNamespace string
	TemplateRenderer appContainerTemplateRenderer
	HttpHelper       appContainerHttpHelper
}

type appContainerRepository struct {
	WorkflowRepository workflow_repository.IWorkflowRepository
	ActivityRepository activity_repository.IActivityRepository
	LogsRepository     logs_repository.ILogsRepository
	MetricsRepository  metrics_repository.IMetricsRepository
	StoragesRepository storages_repository.IStorageRepository
}

type appContainerConnector struct {
	K8sConnector connector.IConnector
}

type appContainerTemplateRenderer struct {
	RenderViewProvider http_render_view.HttpRenderViewProvider
}

type appContainerHttpHelper struct {
	WriteJson func(w http.ResponseWriter, data interface{})
}

func MakeAppContainer() AppContainer {

	// Create the repository instances
	workflowRepository := workflow_repository.New()
	activityRepository := activity_repository.New()
	logsRepository := logs_repository.New()
	metricsRepository := metrics_repository.New()
	storagesRepository := storages_repository.New()

	// create the Connector instances
	k8sConnector := connector.New()

	renderViewprovider := http_render_view.New()

	return AppContainer{
		DefaultNamespace: DEFAULT_NAMESPACE,
		Repository: appContainerRepository{
			WorkflowRepository: workflowRepository,
			ActivityRepository: activityRepository,
			LogsRepository:     logsRepository,
			MetricsRepository:  metricsRepository,
			StoragesRepository: storagesRepository,
		},
		Connector: appContainerConnector{
			K8sConnector: k8sConnector,
		},
		TemplateRenderer: appContainerTemplateRenderer{
			RenderViewProvider: renderViewprovider,
		},
		HttpHelper: appContainerHttpHelper{
			WriteJson: http_helper.WriteJson,
		},
	}
}

// singleton appContainer
var appContainer AppContainer

func App() AppContainer {
	if appContainer.DefaultNamespace == "" {
		appContainer = MakeAppContainer()
	}
	return appContainer
}

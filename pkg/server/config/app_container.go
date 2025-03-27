package config

import (
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/config/http_helper"
	"github.com/ovvesley/akoflow/pkg/server/config/http_render_view"
	"github.com/ovvesley/akoflow/pkg/server/config/logger"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_sdumont"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_singularity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

const DEFAULT_NAMESPACE = "akoflow"
const LOG_FILE_PATH = "akoflow.log"

type AppContainer struct {
	Repository       AppContainerRepository
	Connector        AppContainerConnector
	DefaultNamespace string
	TemplateRenderer AppContainerTemplateRenderer
	HttpHelper       AppContainerHttpHelper
	Logger           *logger.Logger
}

type AppContainerRepository struct {
	WorkflowRepository workflow_repository.IWorkflowRepository
	ActivityRepository activity_repository.IActivityRepository
	LogsRepository     logs_repository.ILogsRepository
	MetricsRepository  metrics_repository.IMetricsRepository
	StoragesRepository storages_repository.IStorageRepository
}

type AppContainerConnector struct {
	K8sConnector         connector_k8s.IConnector
	SingularityConnector connector_singularity.IConnectorSingularity
	SDumontConnector     connector_sdumont.IConnectorSDumont
}

type AppContainerTemplateRenderer struct {
	RenderViewProvider http_render_view.HttpRenderViewProvider
}

type AppContainerHttpHelper struct {
	WriteJson   func(w http.ResponseWriter, data interface{})
	GetUrlParam func(r *http.Request, key string) string
}

func MakeAppContainer() AppContainer {

	// Create the repository instances
	workflowRepository := workflow_repository.New()
	activityRepository := activity_repository.New()
	logsRepository := logs_repository.New()
	metricsRepository := metrics_repository.New()
	storagesRepository := storages_repository.New()

	// create the Connector instances
	k8sConnector := connector_k8s.New()
	singularityConnector := connector_singularity.New()
	sdumontConnector := connector_sdumont.New()

	renderViewprovider := http_render_view.New()

	logger, _ := logger.NewLogger(LOG_FILE_PATH)

	return AppContainer{
		DefaultNamespace: DEFAULT_NAMESPACE,
		Repository: AppContainerRepository{
			WorkflowRepository: workflowRepository,
			ActivityRepository: activityRepository,
			LogsRepository:     logsRepository,
			MetricsRepository:  metricsRepository,
			StoragesRepository: storagesRepository,
		},
		Connector: AppContainerConnector{
			K8sConnector:         k8sConnector,
			SingularityConnector: singularityConnector,
			SDumontConnector:     sdumontConnector,
		},
		TemplateRenderer: AppContainerTemplateRenderer{
			RenderViewProvider: renderViewprovider,
		},
		HttpHelper: AppContainerHttpHelper{
			WriteJson:   http_helper.WriteJson,
			GetUrlParam: http_helper.GetUrlPathParam,
		},
		Logger: logger,
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

func SetAppContainer(container AppContainer) {
	appContainer = container
}

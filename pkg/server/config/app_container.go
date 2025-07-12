package config

import (
	"net/http"
	"os"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/config/http_helper"
	"github.com/ovvesley/akoflow/pkg/server/config/http_render_view"
	"github.com/ovvesley/akoflow/pkg/server/config/logger"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_sdumont"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_singularity"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/schedule_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_execution_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
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
	EnvVars          EnvVars
}
type EnvVars struct {
	EnvVars         map[string]string
	EnvVarByRuntime map[string]map[string]string
}

type AppContainerRepository struct {
	WorkflowRepository          workflow_repository.IWorkflowRepository
	ActivityRepository          activity_repository.IActivityRepository
	LogsRepository              logs_repository.ILogsRepository
	MetricsRepository           metrics_repository.IMetricsRepository
	StoragesRepository          storages_repository.IStorageRepository
	RuntimeRepository           runtime_repository.IRuntimeRepository
	NodeRepository              node_repository.INodeRepository
	NodeMetricsRepository       node_metrics_repository.INodeMetricsRepository
	WorkflowExecutionRepository workflow_execution_repository.IWorkflowExecutionRepository
	ScheduleRepository          schedule_repository.IScheduleRepository
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
	ReadJson    func(r *http.Request, data interface{}) error
}

// GetEnvVars returns the environment variables as a map
func GetEnvVars() (map[string]string, map[string]map[string]string) {
	envVars := make(map[string]string)
	envVarByRuntime := make(map[string]map[string]string)

	runtimes_avaibles := []string{"k8s", "singularity", "sdumont"}

	for _, v := range os.Environ() {
		splitted := strings.Split(v, "=")
		for _, runtime := range runtimes_avaibles {

			envVar := os.Getenv(splitted[0])
			envVars[splitted[0]] = envVar

			if strings.Contains(strings.ToLower(splitted[0]), runtime) {

				currentRuntime := strings.ToLower(splitted[0])
				stringRuntimeSplitted := strings.Split(currentRuntime, "_")
				runtime := stringRuntimeSplitted[0]
				runtime = strings.ToLower(runtime)

				if envVarByRuntime[runtime] == nil {
					envVarByRuntime[runtime] = make(map[string]string)
					envVarByRuntime[runtime][splitted[0]] = envVar
				} else {
					envVarByRuntime[runtime][splitted[0]] = envVar
				}

			}
		}
	}
	return envVars, envVarByRuntime
}

func MakeAppContainer() AppContainer {

	// Create the repository instances
	workflowRepository := workflow_repository.New()
	activityRepository := activity_repository.New()
	logsRepository := logs_repository.New()
	metricsRepository := metrics_repository.New()
	storagesRepository := storages_repository.New()
	runtimeRepository := runtime_repository.New()
	nodeRepository := node_repository.New()
	nodesMetricsRepository := node_metrics_repository.New()
	workflowExecutionRepository := workflow_execution_repository.New()
	scheduleRepository := schedules_repository.New()

	// create the Connector instances
	k8sConnector := connector_k8s.New()
	singularityConnector := connector_singularity.New()
	sdumontConnector := connector_sdumont.New()

	renderViewprovider := http_render_view.New()

	logger, _ := logger.NewLogger(LOG_FILE_PATH)

	envVars, envVarByRuntime := GetEnvVars()

	appContainer := AppContainer{
		DefaultNamespace: DEFAULT_NAMESPACE,
		Repository: AppContainerRepository{
			WorkflowRepository:          workflowRepository,
			ActivityRepository:          activityRepository,
			LogsRepository:              logsRepository,
			MetricsRepository:           metricsRepository,
			StoragesRepository:          storagesRepository,
			RuntimeRepository:           runtimeRepository,
			NodeRepository:              nodeRepository,
			NodeMetricsRepository:       nodesMetricsRepository,
			WorkflowExecutionRepository: workflowExecutionRepository,
			ScheduleRepository:          scheduleRepository,
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
			ReadJson:    http_helper.ReadJson,
		},
		Logger: logger,
		EnvVars: EnvVars{
			EnvVars:         envVars,
			EnvVarByRuntime: envVarByRuntime,
		},
	}
	return appContainer
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

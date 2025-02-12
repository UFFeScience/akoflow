package worker

import (
	"errors"
	"testing"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/config/logger"
	"github.com/ovvesley/akoflow/pkg/server/engine/channel"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_delete_file"
	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_read_file"
	"github.com/ovvesley/akoflow/tests/mocks/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/tests/mocks/pkg/server/repository/workflow_repository"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

func TestStartWorker_ActivityNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWorkflowRepository := workflow_repository.NewMockIWorkflowRepository(ctrl)
	mockActivityRepository := activity_repository.NewMockIActivityRepository(ctrl)

	mockActivityRepository.EXPECT().Find(1).Return(workflow_activity_entity.WorkflowActivities{
		WorkflowId: 1,
	}, errors.New("Activity not found"))
	mockWorkflowRepository.EXPECT().Find(1).Return(workflow_entity.Workflow{}, nil)

	loggerAkoFlow, _ := logger.NewLogger("/tmp/akoflow.log")

	config.SetAppContainer(config.AppContainer{
		Repository: config.AppContainerRepository{
			WorkflowRepository: mockWorkflowRepository,
			ActivityRepository: mockActivityRepository,
		},
		DefaultNamespace: "test",
		Logger:           loggerAkoFlow,
	})

	managerChannel := channel.GetInstance()

	managerChannel.WorfklowChannel <- channel.DataChannel{Id: 1}
	managerChannel.WorfklowChannel <- channel.DataChannel{Id: FLAG_ID_WORKER_STOP_LISTENING}

	worker := New()
	worker.StartWorker()
	contentsLog := utils_read_file.New().ReadFile("/tmp/akoflow.log")

	assert.Contains(t, contentsLog, "WORKER: Activity not found 1")
	defer utils_delete_file.New().DeleteFile("/tmp/akoflow.log")
}

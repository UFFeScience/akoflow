package worker

import (
	"errors"
	"testing"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/engine/channel"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/tests/mocks/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/tests/mocks/pkg/server/repository/workflow_repository"
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

	config.SetAppContainer(config.AppContainer{
		Repository: config.AppContainerRepository{
			WorkflowRepository: mockWorkflowRepository,
			ActivityRepository: mockActivityRepository,
		},
		DefaultNamespace: "test",
	})

	managerChannel := channel.GetInstance()

	managerChannel.WorfklowChannel <- channel.DataChannel{Id: 1}
	managerChannel.WorfklowChannel <- channel.DataChannel{Id: FLAG_ID_WORKER_STOP_LISTENING}

	worker := New()
	worker.StartWorker()

	// verify if activity not found
	// verify if worker is listening

}

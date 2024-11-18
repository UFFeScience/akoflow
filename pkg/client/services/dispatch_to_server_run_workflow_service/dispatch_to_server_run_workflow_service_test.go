package dispatch_to_server_run_workflow_service

import (
	"testing"

	"github.com/ovvesley/akoflow/pkg/client/utils/utils_create_file"
	"github.com/ovvesley/akoflow/tests/mocks/pkg/client/connector/server_connector"
	"github.com/ovvesley/akoflow/tests/mocks/pkg/client/connector/server_connector/server_connector_workflow"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

func TestDispatchToServerRunWorkflowService_Run(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockServerConnector := server_connector.NewMockIServerConnector(ctrl)
	mockServerConnectorWorkflow := server_connector_workflow.NewMockIWorkflow(ctrl)
	mockServerConnector.EXPECT().Workflow().Return(mockServerConnectorWorkflow)

	mockServerConnectorWorkflow.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return()

	defer ctrl.Finish()

	file := utils_create_file.New().CreateTempFile("content")

	dispatchToServerRunWorkflowService := New(mockServerConnector)
	dispatchToServerRunWorkflowService.SetHost("localhost")
	dispatchToServerRunWorkflowService.SetPort("8080")
	dispatchToServerRunWorkflowService.SetFile(file)

	dispatchToServerRunWorkflowService.Run()

}

func TestDispatchToServerRunWorkflowService_SetHost(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockServerConnector := server_connector.NewMockIServerConnector(ctrl)

	defer ctrl.Finish()

	dispatchToServerRunWorkflowService := New(mockServerConnector)
	dispatchToServerRunWorkflowService.SetHost("localhost")

	assert.Equal(t, "localhost", dispatchToServerRunWorkflowService.GetHost())
}

func TestDispatchToServerRunWorkflowService_SetPort(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockServerConnector := server_connector.NewMockIServerConnector(ctrl)

	defer ctrl.Finish()

	dispatchToServerRunWorkflowService := New(mockServerConnector)
	dispatchToServerRunWorkflowService.SetPort("8080")

	assert.Equal(t, "8080", dispatchToServerRunWorkflowService.GetPort())
}

func TestDispatchToServerRunWorkflowService_SetFile(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockServerConnector := server_connector.NewMockIServerConnector(ctrl)

	defer ctrl.Finish()

	dispatchToServerRunWorkflowService := New(mockServerConnector)
	dispatchToServerRunWorkflowService.SetFile("file")

	assert.Equal(t, "file", dispatchToServerRunWorkflowService.GetFile())
}

func TestDispatchToServerRunWorkflowService_GetBase64FileContent(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockServerConnector := server_connector.NewMockIServerConnector(ctrl)

	defer ctrl.Finish()

	dispatchToServerRunWorkflowService := New(mockServerConnector)

	file := utils_create_file.New().CreateTempFile("content")

	base64FileContent := dispatchToServerRunWorkflowService.getBase64FileContent(file)

	assert.Equal(t, "Y29udGVudA==", base64FileContent)
}

func TestDispatchToServerRunWorkflowService_GetFileContent(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockServerConnector := server_connector.NewMockIServerConnector(ctrl)

	defer ctrl.Finish()

	dispatchToServerRunWorkflowService := New(mockServerConnector)

	file := utils_create_file.New().CreateTempFile("content")

	fileContent := dispatchToServerRunWorkflowService.getFileContent(file)

	assert.Equal(t, "content", fileContent)
}

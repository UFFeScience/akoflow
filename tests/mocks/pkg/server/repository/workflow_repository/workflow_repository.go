// Code generated by MockGen. DO NOT EDIT.
// Source: /app/pkg/server/database/repository/workflow_repository/workflow_repository.go
//
// Generated by this command:
//
//	mockgen -source=/app/pkg/server/database/repository/workflow_repository/workflow_repository.go -destination=tests/mocks/pkg/server/database/repository/workflow_repository/workflow_repository.go -package=workflow_repository workflow_repository
//

// Package workflow_repository is a generated GoMock package.
package workflow_repository

import (
	reflect "reflect"

	workflow_entity "github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	workflow_repository "github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	gomock "go.uber.org/mock/gomock"
)

// MockIWorkflowRepository is a mock of IWorkflowRepository interface.
type MockIWorkflowRepository struct {
	ctrl     *gomock.Controller
	recorder *MockIWorkflowRepositoryMockRecorder
	isgomock struct{}
}

// MockIWorkflowRepositoryMockRecorder is the mock recorder for MockIWorkflowRepository.
type MockIWorkflowRepositoryMockRecorder struct {
	mock *MockIWorkflowRepository
}

// NewMockIWorkflowRepository creates a new mock instance.
func NewMockIWorkflowRepository(ctrl *gomock.Controller) *MockIWorkflowRepository {
	mock := &MockIWorkflowRepository{ctrl: ctrl}
	mock.recorder = &MockIWorkflowRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIWorkflowRepository) EXPECT() *MockIWorkflowRepositoryMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockIWorkflowRepository) Create(namespace string, workflow workflow_entity.Workflow) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", namespace, workflow)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockIWorkflowRepositoryMockRecorder) Create(namespace, workflow any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockIWorkflowRepository)(nil).Create), namespace, workflow)
}

// Find mocks base method.
func (m *MockIWorkflowRepository) Find(workflowId int) (workflow_entity.Workflow, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Find", workflowId)
	ret0, _ := ret[0].(workflow_entity.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Find indicates an expected call of Find.
func (mr *MockIWorkflowRepositoryMockRecorder) Find(workflowId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Find", reflect.TypeOf((*MockIWorkflowRepository)(nil).Find), workflowId)
}

// GetPendingWorkflows mocks base method.
func (m *MockIWorkflowRepository) GetPendingWorkflows(namespace string) ([]workflow_entity.Workflow, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPendingWorkflows", namespace)
	ret0, _ := ret[0].([]workflow_entity.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPendingWorkflows indicates an expected call of GetPendingWorkflows.
func (mr *MockIWorkflowRepositoryMockRecorder) GetPendingWorkflows(namespace any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPendingWorkflows", reflect.TypeOf((*MockIWorkflowRepository)(nil).GetPendingWorkflows), namespace)
}

// ListAllWorkflows mocks base method.
func (m *MockIWorkflowRepository) ListAllWorkflows(params *workflow_repository.ListAllWorkflowParams) ([]workflow_entity.Workflow, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListAllWorkflows", params)
	ret0, _ := ret[0].([]workflow_entity.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListAllWorkflows indicates an expected call of ListAllWorkflows.
func (mr *MockIWorkflowRepositoryMockRecorder) ListAllWorkflows(params any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAllWorkflows", reflect.TypeOf((*MockIWorkflowRepository)(nil).ListAllWorkflows), params)
}

// UpdateStatus mocks base method.
func (m *MockIWorkflowRepository) UpdateStatus(id, status int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateStatus", id, status)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateStatus indicates an expected call of UpdateStatus.
func (mr *MockIWorkflowRepositoryMockRecorder) UpdateStatus(id, status any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateStatus", reflect.TypeOf((*MockIWorkflowRepository)(nil).UpdateStatus), id, status)
}

// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/client/connector/server_connector/server_connector.go
//
// Generated by this command:
//
//	mockgen -source=pkg/client/connector/server_connector/server_connector.go -destination=tests/mocks/pkg/client/connector/server_connector/server_connector_mock.go -package=server_connector
//

// Package server_connector is a generated GoMock package.
package server_connector

import (
	reflect "reflect"

	server_connector_workflow "github.com/ovvesley/akoflow/pkg/client/connector/server_connector/server_connector_workflow"
	gomock "go.uber.org/mock/gomock"
)

// MockIServerConnector is a mock of IServerConnector interface.
type MockIServerConnector struct {
	ctrl     *gomock.Controller
	recorder *MockIServerConnectorMockRecorder
	isgomock struct{}
}

// MockIServerConnectorMockRecorder is the mock recorder for MockIServerConnector.
type MockIServerConnectorMockRecorder struct {
	mock *MockIServerConnector
}

// NewMockIServerConnector creates a new mock instance.
func NewMockIServerConnector(ctrl *gomock.Controller) *MockIServerConnector {
	mock := &MockIServerConnector{ctrl: ctrl}
	mock.recorder = &MockIServerConnectorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIServerConnector) EXPECT() *MockIServerConnectorMockRecorder {
	return m.recorder
}

// Workflow mocks base method.
func (m *MockIServerConnector) Workflow() server_connector_workflow.IWorkflow {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Workflow")
	ret0, _ := ret[0].(server_connector_workflow.IWorkflow)
	return ret0
}

// Workflow indicates an expected call of Workflow.
func (mr *MockIServerConnectorMockRecorder) Workflow() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Workflow", reflect.TypeOf((*MockIServerConnector)(nil).Workflow))
}
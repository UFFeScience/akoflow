// Code generated by MockGen. DO NOT EDIT.
// Source: /app/pkg/server/repository/metrics_repository/metrics_repository.go
//
// Generated by this command:
//
//	mockgen -source=/app/pkg/server/repository/metrics_repository/metrics_repository.go -destination=tests/mocks/pkg/server/repository/metrics_repository/metrics_repository.go -package=metrics_repository metrics_repository
//

// Package metrics_repository is a generated GoMock package.
package metrics_repository

import (
	reflect "reflect"

	metrics_repository "github.com/ovvesley/akoflow/pkg/server/repository/metrics_repository"
	gomock "go.uber.org/mock/gomock"
)

// MockIMetricsRepository is a mock of IMetricsRepository interface.
type MockIMetricsRepository struct {
	ctrl     *gomock.Controller
	recorder *MockIMetricsRepositoryMockRecorder
	isgomock struct{}
}

// MockIMetricsRepositoryMockRecorder is the mock recorder for MockIMetricsRepository.
type MockIMetricsRepositoryMockRecorder struct {
	mock *MockIMetricsRepository
}

// NewMockIMetricsRepository creates a new mock instance.
func NewMockIMetricsRepository(ctrl *gomock.Controller) *MockIMetricsRepository {
	mock := &MockIMetricsRepository{ctrl: ctrl}
	mock.recorder = &MockIMetricsRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIMetricsRepository) EXPECT() *MockIMetricsRepositoryMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockIMetricsRepository) Create(params metrics_repository.ParamsMetricsCreate) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", params)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockIMetricsRepositoryMockRecorder) Create(params any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockIMetricsRepository)(nil).Create), params)
}
package server_connector

import (
	"github.com/ovvesley/akoflow/pkg/client/connector/server_connector/server_connector_workflow"
)

type IServerConnector interface {
	Workflow() server_connector_workflow.IWorkflow
}

type ServerConnector struct {
}

func New() *ServerConnector {
	return &ServerConnector{}
}

func (s *ServerConnector) Workflow() server_connector_workflow.IWorkflow {
	return server_connector_workflow.New()
}

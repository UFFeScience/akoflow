package server_connector

import (
	"github.com/UFFeScience/akoflow/internal/cli/api/server_connector/server_connector_workflow"
)

type Connector interface {
	Workflow() server_connector_workflow.Client
}

type ServerConnector struct {
}

func New() *ServerConnector {
	return &ServerConnector{}
}

func (s *ServerConnector) Workflow() server_connector_workflow.Client {
	return server_connector_workflow.New()
}

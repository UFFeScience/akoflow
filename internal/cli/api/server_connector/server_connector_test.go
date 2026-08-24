package server_connector

import "testing"

func TestConnectorProvidesWorkflowClient(t *testing.T) {
	connector := New()
	if connector.Workflow() == nil {
		t.Fatal("workflow connector is nil")
	}
}

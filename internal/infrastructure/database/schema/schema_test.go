package schema

import (
	"strings"
	"testing"
)

func TestCanonicalSchemaIsEmbedded(t *testing.T) {
	for _, definition := range []string{
		"CREATE TABLE schema_metadata",
		"CREATE TABLE environments",
		"CREATE TABLE environment_connection_checks",
		"CREATE TABLE audit_events",
		"CREATE TABLE console_commands",
		"CREATE TABLE console_session_logs",
		"CREATE TABLE workflow_definitions",
		"CREATE TABLE execution_runs",
		"log TEXT NOT NULL DEFAULT ''",
		"CREATE TABLE queue_jobs",
		"CREATE TABLE artifact_versions",
		"CREATE TABLE artifact_materializations",
		"CREATE TABLE storage_download_runs",
		"CREATE TABLE storage_index_runs",
		"CREATE TABLE transfer_runs",
		"CREATE TABLE data_transfer_observations",
	} {
		if !strings.Contains(SQL, definition) {
			t.Errorf("canonical schema does not contain %q", definition)
		}
	}
}

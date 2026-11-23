package environment

import (
	"context"
	"testing"
)

func TestDeleteConnectionChecksCurrentDependenciesAndPreservesOtherConnections(t *testing.T) {
	ctx := context.Background()
	repository := setupRepository(t)
	statements := []string{
		"INSERT INTO environments (id,name) VALUES ('env','Test')",
		"INSERT INTO environment_connections (id,environment_id,name,type,credential_ref) VALUES ('unused','env','Unused','ssh','shared'),('used','env','Used','ssh','shared')",
		"INSERT INTO environment_versions (id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash) VALUES ('v1','env',1,'published','real','none','default','test')",
		`INSERT INTO environment_runtimes (id,environment_version_id,name,driver,mode,configuration) VALUES ('runtime','v1','Execution runtime','ssh','execution','{"connectionId":"used"}')`,
		"INSERT INTO environment_connection_checks (id,connection_id,status,message,latency_ms,checked_at) VALUES ('history','unused','online','',0,CURRENT_TIMESTAMP)",
	}
	for _, statement := range statements {
		if _, err := repository.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := repository.PreviewConnectionDeletion(ctx, "unused")
	if err != nil || preview.Name != "Unused" || len(preview.Dependencies) != 0 {
		t.Fatalf("unexpected deletion preview: %+v, %v", preview, err)
	}
	// A reference created after the preview must still prevent deletion.
	if _, err := repository.db.Exec(`UPDATE environment_runtimes SET configuration='{"connectionId":"unused"}' WHERE id='runtime'`); err != nil {
		t.Fatal(err)
	}
	blocked, err := repository.DeleteConnection(ctx, "unused")
	if err != nil || len(blocked.Dependencies) != 1 || blocked.Dependencies[0].Name != "Execution runtime" {
		t.Fatalf("expected current runtime dependency: %+v, %v", blocked, err)
	}
	if _, err := repository.db.Exec(`UPDATE environment_runtimes SET configuration='{"connectionId":"used"}' WHERE id='runtime'`); err != nil {
		t.Fatal(err)
	}
	deleted, err := repository.DeleteConnection(ctx, "unused")
	if err != nil || len(deleted.Dependencies) != 0 {
		t.Fatalf("delete failed: %+v, %v", deleted, err)
	}
	connections, err := repository.ListConnections(ctx, "env")
	if err != nil || len(connections) != 1 || connections[0].ID != "used" || connections[0].CredentialRef != "shared" {
		t.Fatalf("unrelated connection changed: %+v, %v", connections, err)
	}
	checks, err := repository.ListConnectionChecks(ctx, "unused", 5)
	if err != nil || len(checks) != 0 {
		t.Fatalf("connection history survived deletion: %+v, %v", checks, err)
	}
}

func TestDeleteConnectionBlocksActiveConsoleSession(t *testing.T) {
	ctx := context.Background()
	repository := setupRepository(t)
	statements := []string{
		"INSERT INTO environments (id,name) VALUES ('env','Test')",
		"INSERT INTO environment_connections (id,environment_id,name,type) VALUES ('ssh','env','SSH','ssh')",
		"INSERT INTO environment_versions (id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash) VALUES ('v1','env',1,'published','real','none','default','test')",
		"INSERT INTO resources (id,environment_version_id,type,name,provider_id) VALUES ('resource','v1','local','Host','host')",
		"INSERT INTO console_sessions (id,resource_id,runtime_id,connection_id,status,created_at) VALUES ('console','resource','runtime','ssh','connected',CURRENT_TIMESTAMP)",
	}
	for _, statement := range statements {
		if _, err := repository.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	blocked, err := repository.DeleteConnection(ctx, "ssh")
	if err != nil || len(blocked.Dependencies) != 1 || blocked.Dependencies[0].ID != "console" {
		t.Fatalf("expected active session dependency: %+v, %v", blocked, err)
	}
	if _, err := repository.db.Exec("UPDATE console_sessions SET status='closed' WHERE id='console'"); err != nil {
		t.Fatal(err)
	}
	deleted, err := repository.DeleteConnection(ctx, "ssh")
	if err != nil || len(deleted.Dependencies) != 0 {
		t.Fatalf("closed session blocked deletion: %+v, %v", deleted, err)
	}
}

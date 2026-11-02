package environment

import (
	"context"
	"testing"
)

func TestSummariesPreserveHistoricalScopeAndLatestHealthWithoutSecrets(t *testing.T) {
	r := setupRepository(t)
	for _, query := range []string{
		`INSERT INTO environments(id,name) VALUES ('env','Environment'),('empty','Empty')`,
		`INSERT INTO environment_versions(id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash)
		 VALUES ('old','env',1,'published','','','','old'),('new','env',2,'published','','','','new')`,
		`INSERT INTO resources(id,environment_version_id,type,name,provider_id,cpu_cores,schedulable)
		 VALUES ('old-node','old','local_machine','Old','old',4,1),('new-node','new','local_machine','New','new',8,0)`,
		`INSERT INTO environment_runtimes(id,environment_version_id,name,driver,mode)
		 VALUES ('runtime','old','Local','local','execution')`,
		`INSERT INTO resource_runtime_bindings(resource_id,runtime_id) VALUES ('old-node','runtime')`,
		`INSERT INTO execution_scopes(id,name) VALUES ('scope','Historical scope')`,
		`INSERT INTO execution_scope_environments(execution_scope_id,environment_version_id) VALUES ('scope','old')`,
		`INSERT INTO environment_connections(id,environment_id,name,type,credential_ref,configuration)
		 VALUES ('connection','env','Cloud','cloud','credential-ref','{"provider":"gcp","region":"region","private":"hidden"}')`,
		`INSERT INTO environment_connection_checks(id,connection_id,status,checked_at)
		 VALUES ('before','connection','offline','2026-10-20T12:00:00Z'),('latest','connection','online','2026-10-21T12:00:00Z')`,
	} {
		if _, err := r.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if count, err := r.Count(ctx); err != nil || count != 2 {
		t.Fatalf("environment count=%d %v", count, err)
	}
	all, err := r.ListSummaries(ctx, "", "")
	if err != nil || len(all) != 2 {
		t.Fatalf("summaries=%+v, %v", all, err)
	}
	if all[1].Version.ID != "new" || all[1].Resources[0].CPUCores != 8 || all[1].Resources[0].Schedulable {
		t.Fatalf("latest=%+v", all[1])
	}
	old, err := r.ListSummaries(ctx, "scope", "")
	if err != nil || len(old) != 1 || old[0].Version.ID != "old" || len(old[0].RuntimeBindings) != 1 {
		t.Fatalf("historical=%+v, %v", old, err)
	}
	if len(old[0].ConnectionChecks) != 1 || old[0].ConnectionChecks[0].ID != "latest" {
		t.Fatalf("health=%+v", old)
	}
	connection := old[0].Connections[0]
	if connection.CredentialRef != "" || connection.Configuration["private"] != nil || connection.Configuration["provider"] != "gcp" {
		t.Fatalf("summary exposed private configuration")
	}
	missing, err := r.ListSummaries(ctx, "missing", "")
	if err != nil || len(missing) != 0 {
		t.Fatalf("unrelated environments exposed: %+v %v", missing, err)
	}
}

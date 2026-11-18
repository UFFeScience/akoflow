package environment

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestInstallationConnectionHealthOnlyReturnsSafeObservationFields(t *testing.T) {
	repo := setupRepository(t)
	for _, statement := range []string{
		`INSERT INTO environments(id,name) VALUES ('env','Environment')`,
		`INSERT INTO environment_connections(id,environment_id,name,type,endpoint,username,credential_ref)
		 VALUES ('bad','env','Lab','ssh','secret-host','secret-user','secret-ref'),('fresh','env','New','ssh','','','')`,
		`INSERT INTO environment_connection_checks(id,connection_id,status,message,checked_at,metadata)
		 VALUES ('a','bad','online','secret-old','2026-11-15T10:00:00Z','{}'),
		        ('b','bad','offline','secret-message','2026-11-16T10:00:00Z','{"endpoint":"secret-host"}')`,
	} {
		if _, err := repo.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	items, err := repo.InstallationConnectionHealth(context.Background())
	if err != nil || len(items) != 2 || items[0].Status != "unavailable" || items[1].Status != "unknown" {
		t.Fatalf("unexpected connection observations: %+v %v", items, err)
	}
	payload, err := json.Marshal(items)
	if err != nil || strings.Contains(string(payload), "secret") {
		t.Fatalf("unsafe connection observation: %s %v", payload, err)
	}
}

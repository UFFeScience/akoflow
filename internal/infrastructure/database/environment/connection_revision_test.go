package environment

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestConcurrentConnectionUpdatesPreserveFirstAcceptedRevision(t *testing.T) {
	ctx := context.Background()
	store := setupRepository(t)
	if _, err := store.db.Exec("INSERT INTO environments(id,name) VALUES ('env','fixture')"); err != nil {
		t.Fatal(err)
	}
	original := domain.EnvironmentConnection{ID: "connection", EnvironmentID: "env", Name: "SSH", Type: domain.ConnectionType("ssh")}
	if err := store.UpsertConnection(ctx, original); err != nil {
		t.Fatal(err)
	}
	read, err := store.FindConnection(ctx, original.ID)
	if err != nil || read == nil || read.Revision == "" {
		t.Fatalf("original revision was not returned: %+v, %v", read, err)
	}
	type outcome struct {
		connection domain.EnvironmentConnection
		err        error
	}
	results := make(chan outcome, 2)
	var workers sync.WaitGroup
	for _, endpoint := range []string{"first.example", "second.example"} {
		candidate := *read
		candidate.Endpoint = endpoint
		candidate.CredentialRef = endpoint + "-reference"
		workers.Go(func() {
			updated, err := store.UpdateConnectionRevision(ctx, candidate)
			results <- outcome{updated, err}
		})
	}
	workers.Wait()
	close(results)
	var winner domain.EnvironmentConnection
	conflicts := 0
	for result := range results {
		if errors.Is(result.err, ports.ErrConnectionRevisionConflict) {
			conflicts++
		} else if result.err != nil {
			t.Fatal(result.err)
		} else {
			winner = result.connection
		}
	}
	if conflicts != 1 || winner.Revision == "" || winner.Revision == read.Revision {
		t.Fatalf("expected one fresh revision and one conflict, got %+v and %d", winner, conflicts)
	}
	stored, err := store.FindConnection(ctx, original.ID)
	if err != nil || stored.Revision != winner.Revision || stored.Endpoint != winner.Endpoint ||
		stored.CredentialRef != winner.CredentialRef || stored.Configuration[connectionRevisionKey] != nil {
		t.Fatalf("unexpected persisted connection: %+v, %v", stored, err)
	}
}

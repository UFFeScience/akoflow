package instancearchive

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestRestoreRoundTripAndStartupRollback(t *testing.T) {
	for _, scenario := range []string{"success", "corrupt", "incompatible", "initialization failure"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			active := filepath.Join(root, "active.sqlite")
			db, err := database.Open(active)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := database.Bootstrap(ctx, db); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("INSERT INTO environments (id, name) VALUES ('environment', 'Archived name')"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO queue_jobs (id, category, event_type, available_at, created_at)
				VALUES ('job', 'test', 'test', '2099-01-01', CURRENT_TIMESTAMP)`); err != nil {
				t.Fatal(err)
			}
			service, err := New(db, active, filepath.Join(root, "instances"), "", "test")
			if err != nil {
				t.Fatal(err)
			}
			var archive bytes.Buffer
			if err := service.Export(ctx, &archive, false); err != nil {
				t.Fatal(err)
			}
			snapshot, err := service.Import(ctx, &archive, int64(archive.Len()))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("UPDATE environments SET name = 'Live name'"); err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewRestore(ctx, snapshot.ID)
			if err != nil || preview.Current["environments"] != 1 || preview.Replacement["environments"] != 1 || len(preview.Excluded) == 0 {
				t.Fatalf("preview = %+v, %v", preview, err)
			}
			if err := service.ScheduleRestore(ctx, snapshot.ID, preview.Checksum); err != nil {
				t.Fatal(err)
			}
			prepareRestoreScenario(t, filepath.Join(service.root, snapshot.ID), scenario)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			verifyStartupRestore(t, ctx, active, service.root, scenario)
			result, err := service.LastRestore()
			if err != nil || result == nil || (result.Error == "") != (scenario == "success") {
				t.Fatalf("restore result: %+v, %v", result, err)
			}
			want := "Live name"
			if scenario == "success" {
				want = "Archived name"
				if got := restoredEnvironmentName(t, filepath.Join(service.root, result.RecoveryFile)); got != "Live name" {
					t.Fatalf("recovery file contains %q", got)
				}
			}
			if got := restoredEnvironmentName(t, active); got != want {
				t.Fatalf("active name = %q, want %q", got, want)
			}
			assertRestoredQueue(t, active, scenario == "success")

			if _, err := os.Stat(filepath.Join(service.root, restoreMarker)); !os.IsNotExist(err) {
				t.Fatalf("pending marker survived completed restore: %v", err)
			}
		})
	}
}

func verifyStartupRestore(t *testing.T, ctx context.Context, active, root, scenario string) {
	t.Helper()
	initializations := 0
	err := RestoreOnStartup(ctx, active, root, func() error {
		initializations++
		if initializations == 1 && scenario == "initialization failure" {
			if got := restoredEnvironmentName(t, active); got != "Archived name" {
				t.Fatalf("replacement was not installed: %q", got)
			}
			return errors.New("simulated initialization error")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func prepareRestoreScenario(t *testing.T, directory, scenario string) {
	t.Helper()
	if scenario == "corrupt" {
		if err := os.WriteFile(filepath.Join(directory, "database", "akoflow.sqlite"), []byte("corrupt"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "incompatible" {
		path := filepath.Join(directory, "manifest.json")
		manifest, err := readManifest(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest.FormatVersion++
		if err := writeManifest(path, manifest); err != nil {
			t.Fatal(err)
		}
	}
}

func assertRestoredQueue(t *testing.T, active string, applied bool) {
	t.Helper()
	activeDB, err := database.OpenReadOnly(active)
	if err != nil {
		t.Fatal(err)
	}
	defer activeDB.Close()
	wantJob := "pending"
	if applied {
		wantJob = "cancelled"
	}
	var jobStatus string
	if err := activeDB.QueryRow("SELECT status FROM queue_jobs WHERE id = 'job'").Scan(&jobStatus); err != nil || jobStatus != wantJob {
		t.Fatalf("active job = %q, want %q: %v", jobStatus, wantJob, err)
	}

}

func restoredEnvironmentName(t *testing.T, path string) string {
	t.Helper()
	db, err := database.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.QueryRow("SELECT name FROM environments WHERE id = 'environment'").Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

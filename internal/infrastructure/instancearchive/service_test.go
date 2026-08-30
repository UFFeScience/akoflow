package instancearchive

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestExportRedactsCredentialsAndImportKeepsSnapshotIsolated(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "active.sqlite")
	db, err := database.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO environments (id, name) VALUES ('environment', 'Environment')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO environment_connections
		(id, environment_id, name, type, credential_ref, configuration)
		VALUES ('connection', 'environment', 'Connection', 'ssh', 'private-key',
		'{"token":"secret","proxyCommand":"ssh gateway","nested":{"password":"hidden","port":22}}')`); err != nil {
		t.Fatal(err)
	}
	service, err := New(db, databasePath, filepath.Join(root, "instances"), "", "test")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := service.Export(ctx, &output, false); err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	manifest := archiveManifest(t, archive.File)
	if !manifest.Security.CredentialsRedacted || manifest.Security.CredentialsIncluded {
		t.Fatalf("unexpected archive security: %+v", manifest.Security)
	}
	snapshot := archiveDatabase(t, archive.File, root)
	assertRedactedConnection(t, snapshot)
	assertOriginalConnectionUntouched(t, db)

	imported, err := service.Import(ctx, bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if !imported.ReadOnly || imported.CredentialsSet || imported.Status != "snapshot" {
		t.Fatalf("unexpected imported instance: %+v", imported)
	}
	instances, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 2 || instances[0].Status != "active" || instances[1].ID != imported.ID {
		t.Fatalf("unexpected instance catalog: %+v", instances)
	}
	activated, err := service.Activate(ctx, imported.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activated.ID != imported.ID || activated.Status != "active" || ActiveID(service.root) != imported.ID {
		t.Fatalf("unexpected activated instance: %+v", activated)
	}
	instances, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if instances[0].Status != "snapshot" || instances[1].Status != "active" {
		t.Fatalf("unexpected catalog after activation: %+v", instances)
	}
	if _, err := service.Activate(ctx, "missing-instance"); err == nil {
		t.Fatal("expected activating an unknown instance to fail")
	}
	if _, err := service.Activate(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	if ActiveID(service.root) != "default" {
		t.Fatalf("expected default instance to be active, got %q", ActiveID(service.root))
	}
}

func TestActiveIDFallsBackToDefaultForInvalidSelection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, activeInstanceFile), []byte("../outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ActiveID(root); got != "default" {
		t.Fatalf("expected invalid selection to fall back to default, got %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, activeInstanceFile), []byte("missing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ActiveID(root); got != "default" {
		t.Fatalf("expected missing selection to fall back to default, got %q", got)
	}
}

func TestDefaultRootFollowsConfiguredDatabaseDirectory(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("AKOFLOW_INSTANCE_ARCHIVE_ROOT", "")
	t.Setenv("AKOFLOW_ARCHIVE_TEST_ROOT", workspace)
	t.Setenv("AKOFLOW_DATABASE_PATH", `"$AKOFLOW_ARCHIVE_TEST_ROOT/database/akoflow.sqlite"`)
	expected := filepath.Join(workspace, "database", "instances")
	if got := DefaultRoot(); got != expected {
		t.Fatalf("archive root = %q, expected %q", got, expected)
	}
}

func archiveManifest(t *testing.T, files []*zip.File) domaininstance.ArchiveManifest {
	t.Helper()
	for _, file := range files {
		if file.Name != "manifest.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var manifest domaininstance.ArchiveManifest
		if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
			t.Fatal(err)
		}
		return manifest
	}
	t.Fatal("manifest missing")
	return domaininstance.ArchiveManifest{}
}

func archiveDatabase(t *testing.T, files []*zip.File, root string) string {
	t.Helper()
	for _, file := range files {
		if file.Name != "database/akoflow.sqlite" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		path := filepath.Join(root, "exported.sqlite")
		output, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(output, reader); err != nil {
			t.Fatal(err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	t.Fatal("database missing")
	return ""
}

func assertRedactedConnection(t *testing.T, path string) {
	t.Helper()
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var credential, configuration string
	if err := db.QueryRow(`SELECT credential_ref, configuration FROM environment_connections WHERE id='connection'`).Scan(&credential, &configuration); err != nil {
		t.Fatal(err)
	}
	if credential != "" || strings.Contains(configuration, "secret") ||
		strings.Contains(configuration, "hidden") || !strings.Contains(configuration, "redacted") ||
		!strings.Contains(configuration, "proxyCommand") || !strings.Contains(configuration, "22") {
		t.Fatalf("connection was not redacted: credential=%q configuration=%q", credential, configuration)
	}
}

func assertOriginalConnectionUntouched(t *testing.T, db *sql.DB) {
	t.Helper()
	var credential, configuration string
	if err := db.QueryRow(`SELECT credential_ref, configuration FROM environment_connections WHERE id='connection'`).Scan(&credential, &configuration); err != nil {
		t.Fatal(err)
	}
	if credential != "private-key" || !strings.Contains(configuration, "secret") {
		t.Fatalf("active database was modified: credential=%q configuration=%q", credential, configuration)
	}
}

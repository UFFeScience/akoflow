package data

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	appstorage "github.com/UFFeScience/akoflow/internal/application/storage"
	"github.com/UFFeScience/akoflow/internal/domain"
	dbexecution "github.com/UFFeScience/akoflow/internal/infrastructure/database/execution"
	dbstorage "github.com/UFFeScience/akoflow/internal/infrastructure/database/storage"
	"github.com/UFFeScience/akoflow/internal/provider/storage/filesystem"
)

func TestNewWorkspaceFilesCanBeDownloadedWithoutStorageConfiguration(t *testing.T) {
	repository := testRepository(t)
	root := t.TempDir()
	driver, err := filesystem.New(domain.StorageLocal, root)
	if err != nil {
		t.Fatal(err)
	}
	catalog := dbstorage.New(repository.db)
	browser := appstorage.NewBrowserCoordinator(catalog, appstorage.Registry{domain.StorageLocal: driver})
	var storageIDs []string
	for _, run := range []string{"run", "next-run"} {
		if run != "run" {
			if _, err := repository.db.Exec(`INSERT INTO execution_runs(id,schedule_plan_id,mode,status) VALUES (?,'plan','real','running')`, run); err != nil {
				t.Fatal(err)
			}
		}
		for _, activity := range []string{"activity", "next-activity"} {
			if activity == "next-activity" && run == "run" {
				_, err := repository.db.Exec(`INSERT INTO activity_definitions(id,workflow_version_id,activity_type_id,external_id,name,kind,capabilities,command_spec,resource_requirements,policy)
     SELECT 'next-activity',workflow_version_id,activity_type_id,'next-activity','next activity',kind,capabilities,command_spec,resource_requirements,policy FROM activity_definitions WHERE id='activity'`)
				if err != nil {
					t.Fatal(err)
				}
			}
			storageIDs = append(storageIDs, registerAndDownloadWorkspace(t, repository, browser, root, run, activity))
		}
	}
	for i, id := range storageIDs {
		for j := 0; j < i; j++ {
			if id == storageIDs[j] {
				t.Fatal("different workspaces share storage identity")
			}
		}
	}
	var count int
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM storage_resources`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("non-idempotent registration: count=%d err=%v", count, err)
	}
}

func TestWorkspaceStoragePreservesConfiguredStorage(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	_, err := repository.db.Exec(`INSERT INTO storage_resources(id,environment_version_id,name,type,endpoint,configuration,metadata) VALUES('configured','env','configured','pvc','/configured','{}','{"owner":"user"}')`)
	if err != nil {
		t.Fatal(err)
	}
	handle := workspaceFileHandle("run", "activity", "/workspace/activity")
	handle.Metadata = map[string]any{"artifactStorageResourceId": "configured"}
	if err := repository.CatalogArtifacts(ctx, handle); err != nil {
		t.Fatal(err)
	}
	storage, err := dbstorage.New(repository.db).FindStorage(ctx, "configured")
	if err != nil || storage.Endpoint != "/configured" || len(storage.BrowseRoots) != 0 || storage.Metadata["owner"] != "user" {
		t.Fatalf("configured storage modified: %+v err=%v", storage, err)
	}
}

func TestRemoteWorkspaceStorageUsesPersistedConnection(t *testing.T) {
	for _, cloud := range []bool{false, true} {
		t.Run(fmt.Sprint(cloud), func(t *testing.T) {
			repository := testRepository(t)
			ctx := context.Background()
			uri := "ssh://worker/scratch/run/activity"
			if cloud {
				uri += "?cloudInstanceId=vm-generation"
			}
			workspace := domain.ActivityWorkspace{
				ID: "workspace", RunID: "run", ActivityID: "activity",
				ResourceID: "resource", RuntimeID: "remote",
				ConnectionID: "worker-connection", URI: uri,
				ExecutionPath: "/scratch/run/activity", ObservationPath: "/scratch/run/activity",
				Driver: "ssh", State: domain.WorkspaceSealed, Retention: domain.WorkspaceFinal,
			}
			if err := dbexecution.New(repository.db).SaveWorkspace(ctx, workspace); err != nil {
				t.Fatal(err)
			}
			handle := workspaceFileHandle("run", "activity", workspace.ExecutionPath)
			if err := repository.CatalogArtifacts(ctx, handle); err != nil {
				t.Fatal(err)
			}
			locations, err := repository.ListLocations(ctx, "run")
			if err != nil {
				t.Fatal(err)
			}
			storage, err := dbstorage.New(repository.db).FindStorage(ctx, locations[0].StorageResourceID)
			if err != nil || storage.Type != domain.StorageSSH || len(storage.BrowseRoots) != 1 {
				t.Fatalf("storage=%+v err=%v", storage, err)
			}
			if cloud {
				if storage.Metadata["cloudInstanceId"] != "vm-generation" {
					t.Fatal("cloud identity lost")
				}
			} else if storage.Metadata["connectionId"] != "worker-connection" {
				t.Fatal("SSH identity lost")
			}
		})
	}
}

func workspaceFileHandle(run, activity, root string) domain.ActivityHandle {
	return domain.ActivityHandle{
		RunID: run, ActivityID: activity, ResourceID: "resource",
		Artifacts: &domain.ArtifactManifest{
			Attempt: 1, Root: root,
			Files: []domain.ArtifactObservation{{Path: "report.txt", Change: domain.ArtifactCreated}},
		},
	}
}

func TestRecataloguedWorkspaceReplacesLegacyStorageReference(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	handle := workspaceFileHandle("run", "activity", t.TempDir())
	if err := repository.CatalogArtifacts(ctx, handle); err != nil {
		t.Fatal(err)
	}
	_, err := repository.db.Exec(`INSERT INTO storage_resources
 (id,environment_version_id,name,type,endpoint,configuration,metadata)
 VALUES ('legacy','env','legacy','local','/wrong-activity','{}','{"managedBy":"akoflow"}')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.Exec(`UPDATE data_locations SET storage_resource_id='legacy'`); err != nil {
		t.Fatal(err)
	}
	if err := repository.CatalogArtifacts(ctx, handle); err != nil {
		t.Fatal(err)
	}
	locations, err := repository.ListLocations(ctx, "run")
	if err != nil || len(locations) != 1 || locations[0].StorageResourceID == "legacy" {
		t.Fatalf("locations=%+v err=%v", locations, err)
	}
	browser := appstorage.NewBrowserCoordinator(dbstorage.New(repository.db), appstorage.Registry{})
	if _, err := browser.Stat(ctx, locations[0].StorageResourceID, "report.txt"); err == nil {
		t.Fatal("local browsing must remain opt-in")
	}
}

func registerAndDownloadWorkspace(t *testing.T, repository *Repository, browser *appstorage.BrowserCoordinator, root, run, activity string) string {
	t.Helper()
	ctx := context.Background()
	workspaceRoot := filepath.Join(root, run, activity)
	if err := os.MkdirAll(workspaceRoot, 0750); err != nil {
		t.Fatal(err)
	}
	contents := fmt.Sprintf("%s / %s\n", run, activity)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "report.txt"), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	handle := workspaceFileHandle(run, activity, workspaceRoot)
	handle.Artifacts.Files[0].SizeBytes = int64(len(contents))
	for range 2 {
		if err := repository.CatalogArtifacts(ctx, handle); err != nil {
			t.Fatal(err)
		}
	}
	locations, err := repository.ListLocations(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	var location domain.DataLocation
	for _, candidate := range locations {
		if candidate.URI == "file://"+workspaceRoot+"/report.txt" {
			location = candidate
		}
	}
	if location.StorageResourceID == "" {
		t.Fatal("missing generated file location")
	}
	storage, err := dbstorage.New(repository.db).FindStorage(ctx, location.StorageResourceID)
	if err != nil || storage.Endpoint != workspaceRoot || len(storage.BrowseRoots) != 1 || storage.BrowseRoots[0].Path != workspaceRoot {
		t.Fatalf("storage=%+v err=%v", storage, err)
	}
	runID := fmt.Sprintf("download-%s-%s", run, activity)
	if _, err := browser.StartDownload(ctx, storage.ID, "report.txt", runID); err != nil {
		t.Fatal(err)
	}
	body, _, err := browser.OpenDownload(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(bytes) != contents {
		t.Fatalf("body=%q err=%v", bytes, err)
	}
	if _, err := browser.Stat(ctx, storage.ID, "../activity/report.txt"); err == nil {
		t.Fatal("workspace escape accepted")
	}
	return location.StorageResourceID
}

func TestUnsupportedWorkspaceCannotEnableDaemonFilesystemBrowsing(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	workspace := domain.ActivityWorkspace{
		ID: "workspace", RunID: "run", ActivityID: "activity",
		ResourceID: "resource", RuntimeID: "cluster", URI: "kubernetes://cluster/workspace",
		ExecutionPath: "/workspace", ObservationPath: "/workspace", Driver: "kubernetes",
		State: domain.WorkspaceSealed, Retention: domain.WorkspaceFinal,
	}
	if err := dbexecution.New(repository.db).SaveWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	if err := repository.CatalogArtifacts(ctx, workspaceFileHandle("run", "activity", "/workspace")); err != nil {
		t.Fatal(err)
	}
	locations, err := repository.ListLocations(ctx, "run")
	if err != nil {
		t.Fatal(err)
	}
	storage, err := dbstorage.New(repository.db).FindStorage(ctx, locations[0].StorageResourceID)
	if err != nil || len(storage.BrowseRoots) != 0 {
		t.Fatalf("unsafe browse roots: %+v err=%v", storage, err)
	}
}

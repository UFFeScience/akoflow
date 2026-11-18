package instancearchive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	"github.com/UFFeScience/akoflow/internal/provider"
)

const restoreMarker = ".restore-pending.json"
const restoreResult = ".restore-result.json"

type restoreRequest struct {
	ID       string `json:"id"`
	Checksum string `json:"checksum"`
	Recovery string `json:"recovery"`
}

func (s *Service) PreviewRestore(ctx context.Context, id string) (instance.RestorePreview, error) {
	manifest, source, err := verifiedRestoreSource(ctx, s.root, id)
	if err != nil {
		return instance.RestorePreview{}, err
	}
	db, err := database.OpenReadOnly(source)
	if err != nil {
		return instance.RestorePreview{}, err
	}
	defer db.Close()

	dependencies := map[string]int64{}
	for label, query := range map[string]string{
		"environmentConnections": "SELECT COUNT(*) FROM environment_connections",
		"artifactReferences":     "SELECT COUNT(*) FROM artifact_versions",
		"storageLocations":       "SELECT COUNT(*) FROM storage_resources",
	} {
		var count int64
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return instance.RestorePreview{}, err
		}
		dependencies[label] = count
	}
	return instance.RestorePreview{
		ID: id, Checksum: manifest.Checksums["database/akoflow.sqlite"], ExportedAt: manifest.ExportedAt, ArchiveVersion: manifest.FormatVersion,
		Current: databaseCounts(ctx, s.databasePath), Replacement: databaseCounts(ctx, source),
		Excluded:             []string{"Credential material and Hub sign-in", "External storage and artifact files", "Runtime volumes and container images", "Daemon configuration and installed plugins"},
		ExternalDependencies: dependencies,
	}, nil
}

func (s *Service) ScheduleRestore(ctx context.Context, id, checksum string) error {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	if ActiveID(s.root) != "default" {
		return errors.New("select the writable instance before scheduling a restore")
	}
	marker := filepath.Join(s.root, restoreMarker)
	if _, err := os.Stat(marker); err == nil {
		return errors.New("a restore is already pending")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	manifest, _, err := verifiedRestoreSource(ctx, s.root, id)
	if err != nil {
		return err
	}
	if checksum != manifest.Checksums["database/akoflow.sqlite"] {
		return errors.New("archive changed since preview")
	}
	return writeAtomic(s.root, restoreMarker, func(file *os.File) error {
		return json.NewEncoder(file).Encode(restoreRequest{ID: id, Checksum: manifest.Checksums["database/akoflow.sqlite"], Recovery: provider.NewID("pre-restore") + ".sqlite"})
	})
}

func (s *Service) LastRestore() (*instance.RestoreResult, error) {
	value, err := os.ReadFile(filepath.Join(s.root, restoreResult))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result instance.RestoreResult
	return &result, json.Unmarshal(value, &result)
}

func verifiedRestoreSource(ctx context.Context, root, id string) (instance.ArchiveManifest, string, error) {
	var empty instance.ArchiveManifest
	if !safeArchiveID(id) || id == "default" {
		return empty, "", errors.New("select an imported instance archive")
	}
	directory := filepath.Join(root, id)
	manifest, err := readManifest(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return empty, "", err
	}
	if manifest.Instance.ID != id || manifest.Format != archiveFormat || manifest.FormatVersion != archiveFormatVersion ||
		!manifest.Security.CredentialsRedacted || manifest.Security.CredentialsIncluded {
		return empty, "", errors.New("unsupported or unsafe AkôFlow instance archive")
	}
	source := filepath.Join(directory, "database", "akoflow.sqlite")
	checksum, err := fileChecksum(source)
	if err != nil || checksum != manifest.Checksums["database/akoflow.sqlite"] {
		return empty, "", errors.New("instance database checksum does not match the manifest")
	}
	if err := validateRestoreDatabase(ctx, source); err != nil {
		return empty, "", err
	}
	return manifest, source, nil
}

func validateRestoreDatabase(ctx context.Context, source string) error {
	db, err := database.OpenReadOnly(source)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := database.Validate(ctx, db); err != nil {
		return err
	}
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check(1)").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("SQLite integrity check failed: %s", check)
	}
	return nil
}

func CancelRestoreAfterReset(root string) error {
	if _, err := os.Stat(filepath.Join(root, restoreMarker)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return finishRestore(root, instance.RestoreResult{Error: "Pending restore cancelled by factory reset"})
}

func RestoreOnStartup(ctx context.Context, path, root string, initialize func() error) error {
	marker := filepath.Join(root, restoreMarker)
	value, err := os.ReadFile(marker)
	if errors.Is(err, os.ErrNotExist) {
		return initialize()
	}
	if err != nil {
		return err
	}
	result := instance.RestoreResult{}
	var request restoreRequest
	if err := json.Unmarshal(value, &request); err != nil {
		result.Error = err.Error()
	} else {
		result.RecoveryFile, err = applyRestore(ctx, path, root, request)
		if err != nil {
			result.Error = err.Error()
		}
	}
	if result.Error != "" {
		if err := finishRestore(root, result); err != nil {
			return err
		}
		return initialize()
	}
	if err := initialize(); err != nil {
		result.Error = err.Error()
		if rollbackErr := restoreRecoveryFile(path, root, result.RecoveryFile); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		if err := finishRestore(root, result); err != nil {
			return err
		}
		return initialize()
	}
	return finishRestore(root, result)
}

func applyRestore(ctx context.Context, path, root string, request restoreRequest) (string, error) {
	path, err := database.NormalizePath(path)
	if err != nil {
		return "", err
	}
	manifest, source, err := verifiedRestoreSource(ctx, root, request.ID)
	if err != nil {
		return "", err
	}
	if manifest.Checksums["database/akoflow.sqlite"] != request.Checksum {
		return "", errors.New("archive changed after restore confirmation")
	}
	recovery := request.Recovery
	if !safeArchiveID(recovery) || !strings.HasPrefix(recovery, "pre-restore-") || !strings.HasSuffix(recovery, ".sqlite") {
		return "", errors.New("invalid recovery file name")
	}
	recoveryPath := filepath.Join(root, recovery)
	if _, err := os.Stat(recoveryPath); errors.Is(err, os.ErrNotExist) {
		if err := preserveLiveDatabase(ctx, path, root, recovery); err != nil {
			return "", fmt.Errorf("preserve database before restore: %w", err)
		}
	} else if err != nil {
		return "", err
	}
	if err := clearSQLiteSidecars(path); err != nil {
		return recovery, err
	}
	return recovery, installDatabaseFile(path, source, func(staged string) error { return suspendRestoredWork(ctx, staged) })
}

func preserveLiveDatabase(ctx context.Context, path, root, recovery string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	db, err := database.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	service := Service{db: db}
	return writeAtomic(root, recovery, func(file *os.File) error {
		if err := service.snapshot(ctx, file.Name()); err != nil {
			return err
		}
		return checkpoint(ctx, db)
	})
}

func checkpoint(ctx context.Context, db *sql.DB) error {
	var busy, frames, checkpointed int
	if err := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return errors.New("SQLite is still in use")
	}
	return nil
}

func clearSQLiteSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func installDatabaseFile(path, source string, prepare func(string) error) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	return writeAtomic(filepath.Dir(path), filepath.Base(path), func(file *os.File) error {
		if _, err := io.Copy(file, input); err != nil {
			return err
		}
		if prepare != nil {
			return prepare(file.Name())
		}
		return nil
	})
}

func restoreRecoveryFile(path, root, recovery string) error {
	path, err := database.NormalizePath(path)
	if err != nil {
		return err
	}
	if err := clearSQLiteSidecars(path); err != nil {
		return err
	}
	return installDatabaseFile(path, filepath.Join(root, recovery), nil)
}

func finishRestore(root string, result instance.RestoreResult) error {
	result.CompletedAt = time.Now().UTC()
	if err := writeAtomic(root, restoreResult, func(file *os.File) error {
		return json.NewEncoder(file).Encode(result)
	}); err != nil {
		return err
	}
	return os.Remove(filepath.Join(root, restoreMarker))
}

func suspendRestoredWork(ctx context.Context, path string) error {
	db, err := database.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`UPDATE queue_jobs SET status='cancelled', completed_at=CURRENT_TIMESTAMP, lease_owner='', lease_expires_at=NULL,
			last_error='Interrupted by instance restore' WHERE status IN ('pending','leased')`,
		`UPDATE execution_runs SET status='cancelled', finished_at=CURRENT_TIMESTAMP,
			failure_reason='Interrupted by instance restore' WHERE status IN ('created','running')`,
		`UPDATE task_executions SET status='cancelled', failure_reason='Interrupted by instance restore'
			WHERE status IN ('blocked','ready','queued','preparing','running')`,
		`UPDATE activity_handles SET status='stopped', failure='Interrupted by instance restore' WHERE status IN ('starting','running')`,
		`UPDATE cloud_operation_runs SET status='cancelled', phase='cancelled', finished_at=CURRENT_TIMESTAMP,
			failure_reason='Interrupted by instance restore' WHERE status IN ('queued','running')`,
		`UPDATE transfer_runs SET status='cancelled', error='Interrupted by instance restore'
			WHERE status NOT IN ('completed','failed','cancelled','stopped')`,
		`UPDATE activity_workspaces SET pinned=1 WHERE state != 'released'`,
		`UPDATE execution_schedules SET paused=1 WHERE paused=0`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return checkpoint(ctx, db)
}

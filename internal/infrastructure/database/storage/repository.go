package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type Repository struct{ db *sql.DB }

var sha256Digest = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

func New(db *sql.DB) *Repository { return &Repository{db} }
func (r *Repository) ListEnvironmentStorages(ctx context.Context, environmentID string) ([]domain.StorageResource, error) {
	const query = `SELECT s.id,s.environment_version_id,s.name,s.type,s.endpoint,
		s.capacity_bytes,s.shared,s.read_only,s.configuration,s.credential_reference,s.metadata
		FROM storage_resources s
		JOIN environment_versions v ON v.id=s.environment_version_id
		WHERE v.environment_id=? ORDER BY s.name`
	rows, e := r.db.QueryContext(ctx, query, environmentID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.StorageResource{}
	for rows.Next() {
		v, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}
func (r *Repository) PromoteData(ctx context.Context, storageID, filePath, workflowVersionID, runID, activityID, id string, entry domain.FileEntry, checksum string) error {
	if !sha256Digest.MatchString(checksum) {
		return fmt.Errorf("invalid SHA-256 digest %q", checksum)
	}
	if id == "" {
		id = "data-" + checksum[7:]
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO data_objects(
		id,workflow_version_id,producer_activity_id,logical_name,relative_path,declared,metadata
	) VALUES(?,?,?,?,?,1,'{"source":"storage-browser"}')`,
		id, optionalReference(workflowVersionID), optionalReference(activityID), path.Base(filePath), filePath,
	)
	if e != nil {
		return e
	}
	instance := id + "-instance"
	_, e = tx.ExecContext(ctx, `INSERT INTO data_object_instances(
		id,data_object_id,execution_run_id,producer_activity_id,relative_path,size_bytes,checksum,discovered
	) VALUES(?,?,?,?,?,?,?,1)`,
		instance, id, optionalReference(runID), optionalReference(activityID), filePath, entry.SizeBytes, checksum,
	)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO data_locations(
		id,data_object_instance_id,storage_resource_id,execution_run_id,uri,status
	) VALUES(?,?,?,?,?, 'available')`,
		instance+"-location", instance, storageID, optionalReference(runID),
		fmt.Sprintf("storage://%s/%s", storageID, filePath),
	)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func optionalReference(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func (r *Repository) PromoteArtifact(ctx context.Context, storageID, filePath, id, name, version, scope, scopeID, checksum string, entry domain.FileEntry) error {
	if !sha256Digest.MatchString(checksum) {
		return fmt.Errorf("invalid SHA-256 digest %q", checksum)
	}
	if id == "" {
		id = "artifact-" + checksum[7:]
	}
	if name == "" {
		name = id
	}
	if version == "" {
		version = "1"
	}
	if scope == "" {
		scope = "environment"
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if !validSHA256(checksum) {
		return fmt.Errorf("artifact checksum must be a sha256 digest")
	}
	vid := artifactVersionID(id, version, scope, scopeID)
	variant := vid + "-sif-" + checksum[7:]
	endpoint := "storage-" + storageID
	var environmentID string
	e = tx.QueryRowContext(ctx, `SELECT e.id FROM storage_resources s
		JOIN environment_versions ev ON ev.id=s.environment_version_id
		JOIN environments e ON e.id=ev.environment_id WHERE s.id=?`, storageID).Scan(&environmentID)
	if e != nil {
		return fmt.Errorf("resolve storage environment: %w", e)
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO artifact_versions(id,artifact_id,name,version,scope,scope_id) VALUES(?,?,?,?,?,?)`, vid, id, name, version, scope, scopeID)
	if e != nil {
		return fmt.Errorf("create immutable artifact version: %w", e)
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO artifact_variants(id,artifact_version_id,digest,format,architecture,size_bytes) VALUES(?,?,?,'sif','',?)`, variant, vid, checksum, entry.SizeBytes)
	if e != nil {
		return fmt.Errorf("create immutable artifact variant: %w", e)
	}
	var existingURI, existingEnvironment string
	e = tx.QueryRowContext(ctx, `SELECT uri, environment_id FROM transfer_endpoints WHERE id=?`, endpoint).Scan(&existingURI, &existingEnvironment)
	if e == sql.ErrNoRows {
		_, e = tx.ExecContext(ctx, `INSERT INTO transfer_endpoints(id,kind,uri,environment_id) VALUES(?,?,?,?)`, endpoint, "storage", storageID, environmentID)
		if e != nil {
			return fmt.Errorf("create transfer endpoint: %w", e)
		}
	} else if e != nil {
		return fmt.Errorf("read transfer endpoint: %w", e)
	} else if existingURI != storageID || existingEnvironment != environmentID {
		return fmt.Errorf("transfer endpoint %q conflicts with storage identity", endpoint)
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO artifact_locations(
		id,variant_id,endpoint_id,uri,digest,scope,scope_id,available
	) VALUES(?,?,?,?,?,?,?,1)`,
		variant+"-location", variant, endpoint,
		fmt.Sprintf("storage://%s/%s", storageID, filePath), checksum, scope, scopeID,
	)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func validSHA256(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[len("sha256:"):] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func artifactVersionID(id, version, scope, scopeID string) string {
	return fmt.Sprintf("artifact-version:%s:%s:%s:%s", id, version, scope, scopeID)
}
func (r *Repository) FindStorage(ctx context.Context, id string) (*domain.StorageResource, error) {
	const query = `SELECT id,environment_version_id,name,type,endpoint,capacity_bytes,
		shared,read_only,configuration,credential_reference,metadata
		FROM storage_resources WHERE id=?`
	v, e := scan(r.db.QueryRowContext(ctx, query, id))
	if e == sql.ErrNoRows {
		return nil, nil
	}
	return v, e
}
func scan(s interface{ Scan(...any) error }) (*domain.StorageResource, error) {
	var v domain.StorageResource
	var typ, config, meta string
	e := s.Scan(&v.ID, &v.EnvironmentVersionID, &v.Name, &typ, &v.Endpoint, &v.CapacityBytes, &v.Shared, &v.ReadOnly, &config, &v.CredentialReference, &meta)
	if e != nil {
		return nil, e
	}
	v.Type = domain.StorageType(typ)
	_ = json.Unmarshal([]byte(config), &v.Configuration)
	_ = json.Unmarshal([]byte(meta), &v.Metadata)
	if roots, ok := v.Configuration["browseRoots"].([]any); ok {
		for _, x := range roots {
			b, _ := json.Marshal(x)
			var root domain.StorageBrowseRoot
			_ = json.Unmarshal(b, &root)
			v.BrowseRoots = append(v.BrowseRoots, root)
		}
	}
	if policy, ok := v.Configuration["indexPolicy"]; ok {
		raw, _ := json.Marshal(policy)
		_ = json.Unmarshal(raw, &v.IndexPolicy)
	}
	return &v, nil
}
func (r *Repository) SaveDownload(ctx context.Context, v domain.DownloadRun) error {
	const query = `INSERT INTO storage_download_runs(
		id,storage_resource_id,path,status,strategy,url,size_bytes,transferred_bytes,error,created_at,updated_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		status=excluded.status,url=excluded.url,transferred_bytes=excluded.transferred_bytes,
		error=excluded.error,updated_at=excluded.updated_at`
	_, e := r.db.ExecContext(ctx, query,
		v.ID, v.StorageID, v.Path, v.Status, v.Strategy, v.URL, v.SizeBytes,
		v.TransferredBytes, v.Error, v.CreatedAt, v.UpdatedAt,
	)
	return e
}
func (r *Repository) FindDownload(ctx context.Context, id string) (*domain.DownloadRun, error) {
	var v domain.DownloadRun
	const query = `SELECT id,storage_resource_id,path,status,strategy,url,size_bytes,
		transferred_bytes,error,created_at,updated_at
		FROM storage_download_runs WHERE id=?`
	e := r.db.QueryRowContext(ctx, query, id).Scan(
		&v.ID, &v.StorageID, &v.Path, &v.Status, &v.Strategy, &v.URL,
		&v.SizeBytes, &v.TransferredBytes, &v.Error, &v.CreatedAt, &v.UpdatedAt,
	)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	return &v, e
}
func (r *Repository) SaveIndexRun(ctx context.Context, value domain.IndexRun) error {
	const query = `INSERT INTO storage_index_runs(
		id,storage_resource_id,status,indexed_entries,error,created_at,finished_at
	) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		status=excluded.status,indexed_entries=excluded.indexed_entries,
		error=excluded.error,finished_at=excluded.finished_at`
	_, err := r.db.ExecContext(ctx, query,
		value.ID, value.StorageID, value.Status, value.IndexedEntries,
		value.Error, value.CreatedAt, value.FinishedAt,
	)
	return err
}

func (r *Repository) ListIndexRuns(ctx context.Context, storageID string) ([]domain.IndexRun, error) {
	const query = `SELECT id,storage_resource_id,status,indexed_entries,error,created_at,finished_at
		FROM storage_index_runs WHERE storage_resource_id=? ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, storageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.IndexRun{}
	for rows.Next() {
		var value domain.IndexRun
		var finishedAt sql.NullTime
		if err = rows.Scan(
			&value.ID, &value.StorageID, &value.Status, &value.IndexedEntries,
			&value.Error, &value.CreatedAt, &finishedAt,
		); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			value.FinishedAt = finishedAt.Time
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (_ Repository) _time(_ time.Time) {}

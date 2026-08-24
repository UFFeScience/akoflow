package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Repository struct{ db *sql.DB }

var _ ports.DataCatalog = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CatalogArtifacts(ctx context.Context, handle domain.ActivityHandle) error {
	if handle.Artifacts == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var workflowVersionID, environmentVersionID string
	if err := tx.QueryRowContext(ctx, `SELECT workflow_version_id FROM activity_definitions WHERE id=?`,
		handle.ActivityID).Scan(&workflowVersionID); err != nil {
		return fmt.Errorf("resolve artifact workflow: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT environment_version_id FROM resources WHERE id=?`,
		handle.ResourceID).Scan(&environmentVersionID); err != nil {
		return fmt.Errorf("resolve artifact environment: %w", err)
	}
	storageType := domain.StorageLocal
	locationStatus := domain.DataLocationEphemeral
	if configured, _ := handle.Metadata["artifactStorageType"].(string); configured == "pvc" || configured == "nfs" {
		storageType = domain.StorageType(configured)
		locationStatus = domain.DataLocationAvailable
	}
	storageID := stableID("storage", environmentVersionID, handle.ResourceID, string(storageType))
	configuredStorageID, _ := handle.Metadata["artifactStorageResourceId"].(string)
	if configuredStorageID != "" {
		storageID = configuredStorageID
	} else if _, err := tx.ExecContext(ctx, `INSERT INTO storage_resources (
		id, environment_version_id, name, type, endpoint, shared, configuration, metadata
	) VALUES (?, ?, ?, ?, ?, ?, '{}', '{"managedBy":"akoflow"}')
	ON CONFLICT(id) DO NOTHING`, storageID, environmentVersionID,
		"workspace-"+handle.ResourceID, storageType, handle.Artifacts.Root,
		storageType == domain.StoragePVC || storageType == domain.StorageNFS); err != nil {
		return fmt.Errorf("register workspace storage: %w", err)
	}
	for _, artifact := range handle.Artifacts.Files {
		if artifact.Change == domain.ArtifactDeleted {
			continue
		}
		objectID := stableID("object", workflowVersionID, handle.ActivityID, artifact.Path)
		if _, err := tx.ExecContext(ctx, `INSERT INTO data_objects (
			id, workflow_version_id, producer_activity_id, logical_name, relative_path, declared, metadata
		) VALUES (?, ?, ?, ?, ?, 0, '{"source":"runtime-discovery"}')
		ON CONFLICT(id) DO UPDATE SET relative_path=excluded.relative_path`, objectID,
			workflowVersionID, handle.ActivityID, path.Base(artifact.Path), artifact.Path); err != nil {
			return fmt.Errorf("register data object: %w", err)
		}
		instanceID := stableID("instance", handle.RunID, handle.ActivityID,
			fmt.Sprint(handle.Artifacts.Attempt), artifact.Path)
		metadata, _ := json.Marshal(map[string]any{"change": artifact.Change})
		if _, err := tx.ExecContext(ctx, `INSERT INTO data_object_instances (
			id, data_object_id, execution_run_id, producer_activity_id, attempt,
			relative_path, size_bytes, checksum, discovered, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(execution_run_id, producer_activity_id, attempt, relative_path) DO UPDATE SET
			size_bytes=excluded.size_bytes, checksum=excluded.checksum, metadata=excluded.metadata`,
			instanceID, objectID, handle.RunID, handle.ActivityID, handle.Artifacts.Attempt,
			artifact.Path, artifact.SizeBytes, artifact.Checksum, string(metadata)); err != nil {
			return fmt.Errorf("register data object instance: %w", err)
		}
		locationID := stableID("location", instanceID, handle.ResourceID)
		locationURI := (&url.URL{Scheme: "file", Path: path.Join(handle.Artifacts.Root, artifact.Path)}).String()
		if _, err := tx.ExecContext(ctx, `INSERT INTO data_locations (
			id, data_object_instance_id, storage_resource_id, resource_id,
			execution_run_id, uri, available_at, status, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(data_object_instance_id, uri) DO UPDATE SET
			status=excluded.status, available_at=excluded.available_at`, locationID, instanceID,
			storageID, handle.ResourceID, handle.RunID, locationURI, handle.FinishedAt,
			locationStatus, locationMetadata(locationStatus)); err != nil {
			return fmt.Errorf("register data location: %w", err)
		}
	}
	return tx.Commit()
}

func locationMetadata(status domain.DataLocationStatus) string {
	if status == domain.DataLocationAvailable {
		return `{"lifetime":"storage"}`
	}
	return `{"lifetime":"pod"}`
}

func (r *Repository) ListInstances(ctx context.Context, runID string) ([]domain.DataObjectInstance, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, data_object_id, execution_run_id,
		producer_activity_id, attempt, relative_path, size_bytes, checksum, media_type,
		discovered, metadata FROM data_object_instances WHERE execution_run_id=?
		ORDER BY producer_activity_id, relative_path`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.DataObjectInstance
	for rows.Next() {
		var item domain.DataObjectInstance
		var discovered bool
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.DataObjectID, &item.ExecutionRunID,
			&item.ProducerActivityID, &item.Attempt, &item.RelativePath, &item.SizeBytes,
			&item.Checksum, &item.MediaType, &discovered, &metadata); err != nil {
			return nil, err
		}
		item.Discovered = discovered
		_ = json.Unmarshal(metadata, &item.Metadata)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListLocations(ctx context.Context, runID string) ([]domain.DataLocation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, data_object_instance_id,
		COALESCE(storage_resource_id, ''), COALESCE(resource_id, ''), execution_run_id,
		uri, status, metadata FROM data_locations WHERE execution_run_id=? ORDER BY uri`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.DataLocation
	for rows.Next() {
		var item domain.DataLocation
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.DataObjectInstanceID, &item.StorageResourceID,
			&item.ResourceID, &item.ExecutionRunID, &item.URI, &item.Status, &metadata); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListArtifactLocations(ctx context.Context) ([]domain.ArtifactLocation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, variant_id, endpoint_id, uri, digest, scope, scope_id, available FROM artifact_locations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.ArtifactLocation
	for rows.Next() {
		var v domain.ArtifactLocation
		var available bool
		if err := rows.Scan(&v.ID, &v.VariantID, &v.EndpointID, &v.URI, &v.Digest, &v.Scope, &v.ScopeID, &available); err != nil {
			return nil, err
		}
		v.Available = available
		values = append(values, v)
	}
	return values, rows.Err()
}

func (r *Repository) ListArtifacts(ctx context.Context) ([]domain.ExecutableArtifact, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT artifact_id, name, version FROM artifact_versions ORDER BY artifact_id, version, scope, scope_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.ExecutableArtifact
	for rows.Next() {
		var value domain.ExecutableArtifact
		if err := rows.Scan(&value.ID, &value.Name, &value.Version); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *Repository) ListArtifactMaterializations(ctx context.Context, runID string) ([]domain.ArtifactMaterialization, error) {
	query := `SELECT id, COALESCE(run_id, ''), COALESCE(activity_id, ''), variant_id, digest, resource_id, COALESCE(environment_version_id, ''), destination_path, status, verified_digest FROM artifact_materializations`
	args := []any{}
	if runID != "" {
		query += ` WHERE run_id=?`
		args = append(args, runID)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.ArtifactMaterialization
	for rows.Next() {
		var v domain.ArtifactMaterialization
		if err := rows.Scan(&v.ID, &v.RunID, &v.ActivityID, &v.VariantID, &v.Digest, &v.ResourceID, &v.EnvironmentID, &v.DestinationPath, &v.Status, &v.VerifiedDigest); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func (r *Repository) SaveArtifactMaterialization(ctx context.Context, value domain.ArtifactMaterialization) error {
	const query = `INSERT INTO artifact_materializations (
		id,run_id,activity_id,variant_id,digest,resource_id,environment_version_id,destination_path,status,verified_digest
	) VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		run_id=excluded.run_id,
		activity_id=excluded.activity_id,
		status=excluded.status,
		verified_digest=excluded.verified_digest,
		destination_path=excluded.destination_path,
		updated_at=CURRENT_TIMESTAMP`
	_, err := r.db.ExecContext(ctx, query,
		value.ID, nullable(value.RunID), nullable(value.ActivityID), value.VariantID, value.Digest,
		value.ResourceID, nullable(value.EnvironmentID), value.DestinationPath, value.Status, value.VerifiedDigest,
	)
	return err
}

// SaveTransferRun persists resumable transfer state rather than leaving it in
// the process memory of a materializer worker.
func (r *Repository) SaveTransferRun(ctx context.Context, value domain.DataTransferRun) error {
	verified, err := json.Marshal(value.VerifiedBlobs)
	if err != nil {
		return err
	}
	chunks, err := json.Marshal(value.CompletedChunks)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO transfer_runs(id,plan_id,strategy,status,verified_blobs,completed_chunks,started_at,finished_at,transferred_bytes,error)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status, verified_blobs=excluded.verified_blobs,
		completed_chunks=excluded.completed_chunks, started_at=excluded.started_at, finished_at=excluded.finished_at,
		transferred_bytes=excluded.transferred_bytes, error=excluded.error`, value.ID, value.PlanID, value.Strategy, value.Status, string(verified), string(chunks), value.StartedAt, value.FinishedAt, value.TransferredBytes, value.Error)
	return err
}

func (r *Repository) ListArtifactTransferRuns(ctx context.Context, runID string) ([]domain.DataTransferRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id,t.plan_id,t.strategy,t.status,t.verified_blobs,t.completed_chunks,t.started_at,t.finished_at,t.transferred_bytes,t.error
		FROM transfer_runs t JOIN artifact_materializations m ON t.id='transfer-' || m.id
		WHERE m.run_id=? ORDER BY t.id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.DataTransferRun{}
	for rows.Next() {
		var value domain.DataTransferRun
		var verified, chunks string
		if err := rows.Scan(&value.ID, &value.PlanID, &value.Strategy, &value.Status, &verified, &chunks, &value.StartedAt, &value.FinishedAt, &value.TransferredBytes, &value.Error); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(verified), &value.VerifiedBlobs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(chunks), &value.CompletedChunks); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *Repository) FindTransferRun(ctx context.Context, id string) (*domain.DataTransferRun, error) {
	var value domain.DataTransferRun
	var verified, chunks string
	err := r.db.QueryRowContext(ctx, `SELECT id,plan_id,strategy,status,verified_blobs,completed_chunks,started_at,finished_at,transferred_bytes,error FROM transfer_runs WHERE id=?`, id).
		Scan(&value.ID, &value.PlanID, &value.Strategy, &value.Status, &verified, &chunks, &value.StartedAt, &value.FinishedAt, &value.TransferredBytes, &value.Error)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(verified), &value.VerifiedBlobs); err != nil {
		return nil, fmt.Errorf("decode verified blobs: %w", err)
	}
	if err := json.Unmarshal([]byte(chunks), &value.CompletedChunks); err != nil {
		return nil, fmt.Errorf("decode transfer chunks: %w", err)
	}
	return &value, nil
}

func (r *Repository) SaveArtifactBuild(ctx context.Context, value domain.ArtifactBuild) error {
	const query = `INSERT INTO artifact_builds(
		id, artifact_version_id, source_type, context_digest, recipe_path,
		recipe_digest, target_format, target_os, target_architecture, build_arguments, cache_key
	) VALUES(?,?,?,?,?,?,?,?,?,?,?)`
	_, err := r.db.ExecContext(ctx, query,
		value.ID, value.ArtifactVersionID, value.SourceType, value.ContextDigest,
		value.RecipePath, value.RecipeDigest, value.TargetFormat, value.TargetOS,
		value.TargetArchitecture, value.BuildArguments, value.CacheKey)
	return err
}

func (r *Repository) RegisterArtifactVersion(ctx context.Context, value domain.ArtifactVersion) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO artifact_versions(id,artifact_id,name,version,scope,scope_id)
		VALUES(?,?,?,?,?,?)`, value.ID, value.ArtifactID, value.ArtifactID, value.Version, value.Scope, value.ScopeID)
	return err
}

func (r *Repository) FindArtifactBuildByCacheKey(ctx context.Context, cacheKey string) (*domain.ArtifactBuild, error) {
	return r.findArtifactBuild(ctx, `SELECT id,artifact_version_id,source_type,context_digest,recipe_path,recipe_digest,target_format,target_os,target_architecture,build_arguments,cache_key FROM artifact_builds WHERE cache_key=?`, cacheKey)
}

func (r *Repository) FindArtifactBuild(ctx context.Context, id string) (*domain.ArtifactBuild, error) {
	return r.findArtifactBuild(ctx, `SELECT id,artifact_version_id,source_type,context_digest,recipe_path,recipe_digest,target_format,target_os,target_architecture,build_arguments,cache_key FROM artifact_builds WHERE id=?`, id)
}

func (r *Repository) ListArtifactBuilds(ctx context.Context, artifactID string) ([]domain.ArtifactBuild, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT b.id,b.artifact_version_id,b.source_type,b.context_digest,b.recipe_path,b.recipe_digest,b.target_format,b.target_os,b.target_architecture,b.build_arguments,b.cache_key
		FROM artifact_builds b JOIN artifact_versions v ON v.id=b.artifact_version_id
		WHERE v.artifact_id=? ORDER BY b.created_at DESC`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.ArtifactBuild{}
	for rows.Next() {
		var value domain.ArtifactBuild
		if err := rows.Scan(&value.ID, &value.ArtifactVersionID, &value.SourceType, &value.ContextDigest, &value.RecipePath, &value.RecipeDigest, &value.TargetFormat, &value.TargetOS, &value.TargetArchitecture, &value.BuildArguments, &value.CacheKey); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *Repository) findArtifactBuild(ctx context.Context, query, argument string) (*domain.ArtifactBuild, error) {
	var value domain.ArtifactBuild
	err := r.db.QueryRowContext(ctx, query, argument).
		Scan(&value.ID, &value.ArtifactVersionID, &value.SourceType, &value.ContextDigest, &value.RecipePath, &value.RecipeDigest, &value.TargetFormat, &value.TargetOS, &value.TargetArchitecture, &value.BuildArguments, &value.CacheKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *Repository) SaveBuildRun(ctx context.Context, value domain.BuildRun) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO build_runs(id,artifact_build_id,status,output_variant_id,output_digest,logs,error)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status, output_variant_id=excluded.output_variant_id, output_digest=excluded.output_digest, logs=excluded.logs, error=excluded.error`,
		value.ID, value.ArtifactBuildID, value.Status, nullable(value.OutputVariantID), value.OutputDigest, value.Logs, value.Error)
	return err
}

func (r *Repository) FindBuildRun(ctx context.Context, id string) (*domain.BuildRun, error) {
	var value domain.BuildRun
	err := r.db.QueryRowContext(ctx, `SELECT id,artifact_build_id,status,COALESCE(output_variant_id,''),output_digest,logs,error FROM build_runs WHERE id=?`, id).
		Scan(&value.ID, &value.ArtifactBuildID, &value.Status, &value.OutputVariantID, &value.OutputDigest, &value.Logs, &value.Error)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *Repository) ListBuildRuns(ctx context.Context, buildID string) ([]domain.BuildRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,artifact_build_id,status,COALESCE(output_variant_id,''),output_digest,logs,error FROM build_runs WHERE artifact_build_id=? ORDER BY created_at DESC`, buildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.BuildRun{}
	for rows.Next() {
		var value domain.BuildRun
		if err := rows.Scan(&value.ID, &value.ArtifactBuildID, &value.Status, &value.OutputVariantID, &value.OutputDigest, &value.Logs, &value.Error); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *Repository) SaveBuildContext(ctx context.Context, value domain.BuildContextArtifact) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO build_contexts(digest,storage_uri,size_bytes,media_type) VALUES(?,?,?,?)`, value.Digest, value.StorageURI, value.SizeBytes, value.MediaType)
	return err
}

func (r *Repository) FindBuildContext(ctx context.Context, digest string) (*domain.BuildContextArtifact, error) {
	var value domain.BuildContextArtifact
	err := r.db.QueryRowContext(ctx, `SELECT digest,storage_uri,size_bytes,media_type FROM build_contexts WHERE digest=?`, digest).Scan(&value.Digest, &value.StorageURI, &value.SizeBytes, &value.MediaType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// FindBuildOutput resolves only a published output; queued or failed builds
// are deliberately not executable workflow inputs.
func (r *Repository) FindBuildOutput(ctx context.Context, buildID string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	const query = `SELECT v.id,v.digest,v.format,v.architecture,v.size_bytes,
		l.id,l.variant_id,l.scope,l.scope_id,l.endpoint_id,l.uri,l.digest,l.available
		FROM build_runs r JOIN artifact_variants v ON v.id=r.output_variant_id
		JOIN artifact_locations l ON l.variant_id=v.id
		WHERE r.artifact_build_id=? AND r.status='completed' ORDER BY r.finished_at DESC LIMIT 1`
	var variant domain.ArtifactVariant
	var location domain.ArtifactLocation
	err := r.db.QueryRowContext(ctx, query, buildID).Scan(&variant.ID, &variant.Digest, &variant.Format, &variant.Architecture, &variant.SizeBytes,
		&location.ID, &location.VariantID, &location.Scope, &location.ScopeID, &location.EndpointID, &location.URI, &location.Digest, &location.Available)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &variant, &location, nil
}

// FindDockerBuildOutput locates the latest published SIF for an OCI image.
// It is used to transparently upgrade legacy OCI workflow references when a
// Slurm target requires a materialized executable.
func (r *Repository) FindDockerBuildOutput(ctx context.Context, image, architecture string) (*domain.ArtifactBuild, *domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	const query = `SELECT b.id,b.artifact_version_id,b.source_type,b.context_digest,b.recipe_path,b.recipe_digest,b.target_format,b.target_os,b.target_architecture,b.build_arguments,b.cache_key,
		v.id,v.digest,v.format,v.architecture,v.size_bytes,
		l.id,l.variant_id,l.scope,l.scope_id,l.endpoint_id,l.uri,l.digest,l.available
		FROM artifact_builds b
		JOIN build_runs r ON r.artifact_build_id=b.id
		JOIN artifact_variants v ON v.id=r.output_variant_id
		JOIN artifact_locations l ON l.variant_id=v.id
		WHERE b.source_type='docker-image' AND b.recipe_path=? AND r.status='completed'
		AND (?='' OR b.target_architecture='' OR b.target_architecture=?)
		ORDER BY CASE WHEN b.target_architecture=? THEN 0 ELSE 1 END, r.finished_at DESC LIMIT 1`
	var build domain.ArtifactBuild
	var variant domain.ArtifactVariant
	var location domain.ArtifactLocation
	err := r.db.QueryRowContext(ctx, query, image, architecture, architecture, architecture).Scan(
		&build.ID, &build.ArtifactVersionID, &build.SourceType, &build.ContextDigest, &build.RecipePath, &build.RecipeDigest, &build.TargetFormat, &build.TargetOS, &build.TargetArchitecture, &build.BuildArguments, &build.CacheKey,
		&variant.ID, &variant.Digest, &variant.Format, &variant.Architecture, &variant.SizeBytes,
		&location.ID, &location.VariantID, &location.Scope, &location.ScopeID, &location.EndpointID, &location.URI, &location.Digest, &location.Available,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return &build, &variant, &location, nil
}

// PublishBuildOutput makes a variant visible only together with the completed
// build run and its durable location.
func (r *Repository) PublishBuildOutput(ctx context.Context, runID string, variant domain.ArtifactVariant, location domain.ArtifactLocation) error {
	if variant.ID == "" || location.ID == "" || variant.Digest == "" || variant.Digest != location.Digest {
		return fmt.Errorf("build output must have matching variant and location digests")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var artifactVersionID string
	if err := tx.QueryRowContext(ctx, `SELECT b.artifact_version_id FROM build_runs r JOIN artifact_builds b ON b.id=r.artifact_build_id WHERE r.id=?`, runID).Scan(&artifactVersionID); err != nil {
		return fmt.Errorf("resolve build run: %w", err)
	}
	const insertVariant = `INSERT INTO artifact_variants(
		id, artifact_version_id, digest, format, architecture, size_bytes
	) VALUES(?,?,?,?,?,?)`
	if _, err := tx.ExecContext(ctx, insertVariant,
		variant.ID, artifactVersionID, variant.Digest, variant.Format,
		variant.Architecture, variant.SizeBytes); err != nil {
		return fmt.Errorf("create build variant: %w", err)
	}
	const insertLocation = `INSERT INTO artifact_locations(
		id, variant_id, endpoint_id, uri, digest, scope, scope_id, available
	) VALUES(?,?,?,?,?,?,?,?)`
	// The managed artifact store is a first-class endpoint too; it is not tied
	// to a compute resource or environment.
	if location.EndpointID == "artifact-store" {
		const artifactStoreEndpoint = `INSERT INTO transfer_endpoints(
			id, kind, uri, configuration
		) VALUES('artifact-store', 'artifact-store', 'artifact://', '{}')
		ON CONFLICT(id) DO NOTHING`
		if _, err := tx.ExecContext(ctx, artifactStoreEndpoint); err != nil {
			return fmt.Errorf("register artifact store endpoint: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, insertLocation,
		location.ID, variant.ID, location.EndpointID, location.URI,
		location.Digest, location.Scope, location.ScopeID, location.Available); err != nil {
		return fmt.Errorf("create build location: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE build_runs SET status='completed',output_variant_id=?,output_digest=?,finished_at=CURRENT_TIMESTAMP WHERE id=?`, variant.ID, variant.Digest, runID); err != nil {
		return fmt.Errorf("publish build run: %w", err)
	}
	return tx.Commit()
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stableID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)[:16])
}

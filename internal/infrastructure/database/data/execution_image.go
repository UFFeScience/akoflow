package data

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

var executionImageDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func (r *Repository) CatalogExecutionImage(ctx context.Context, handle domain.ActivityHandle) (map[string]string, error) {
	observation, _ := handle.Metadata["imageObservation"].(map[string]any)
	if observation["status"] != "observed" {
		return nil, nil
	}
	if variant, _ := observation["sifVariantId"].(string); variant != "" {
		digest, _ := observation["sifDigest"].(string)
		return r.catalogUsedSIF(ctx, variant, digest)
	}
	digest, _ := observation["registryDigest"].(string)
	runtimeID, _ := observation["runtimeImageId"].(string)
	localDocker := handle.Metadata["executionTarget"] == "local-docker"
	kind := "oci-manifest"
	if localDocker || !executionImageDigest.MatchString(digest) {
		digest = strings.TrimPrefix(runtimeID, "containerd://")
		digest = strings.TrimPrefix(digest, "docker://")
		kind = "runtime-image"
	}
	if !executionImageDigest.MatchString(digest) {
		return nil, nil
	}
	platform, _ := observation["platform"].(string)
	_, architecture, _ := strings.Cut(platform, "/")
	architecture, _, _ = strings.Cut(architecture, "/")
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	link, err := executionImageVariant(ctx, tx, digest, architecture, platform, kind, localDocker)
	if err != nil {
		return nil, err
	}
	if localDocker {
		if _, err := tx.ExecContext(ctx, `INSERT INTO transfer_endpoints(id,kind,uri) VALUES('local-docker','local','docker-image://') ON CONFLICT(id) DO NOTHING`); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_locations(id,variant_id,endpoint_id,uri,digest,scope)
			SELECT ?,?,'local-docker',?,?,'system'
			WHERE NOT EXISTS (SELECT 1 FROM artifact_locations WHERE variant_id=? AND endpoint_id='local-docker' AND digest=? AND available=1)
			ON CONFLICT(id) DO UPDATE SET available=1`,
			stableID("local-docker", link["variantId"]), link["variantId"], "docker-image://"+digest, digest, link["variantId"], digest); err != nil {
			return nil, err
		}
	}
	return link, tx.Commit()
}

func executionImageVariant(ctx context.Context, tx *sql.Tx, digest, architecture, platform, kind string, localDocker bool) (map[string]string, error) {
	var artifactID, version, variantID string
	query := `SELECT av.artifact_id,av.version,v.id
		FROM artifact_variants v JOIN artifact_versions av ON av.id=v.artifact_version_id
		JOIN artifact_locations l ON l.variant_id=v.id
		WHERE v.digest=? AND v.architecture=? AND v.format='oci'
		AND ((? AND l.endpoint_id='local-docker') OR (NOT ? AND ?='oci-manifest' AND l.uri LIKE 'oci://%'))
		ORDER BY v.id LIMIT 1`
	err := tx.QueryRowContext(ctx, query, digest, architecture, localDocker, localDocker, kind).Scan(&artifactID, &version, &variantID)
	if errors.Is(err, sql.ErrNoRows) {
		artifactID = "execution-" + kind
		version = digest
		if platform != "" {
			version += "/" + platform
		}
		versionID := stableID(artifactID, version)
		variantID = stableID(versionID, digest)
		if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_versions(id,artifact_id,name,version,scope)
			VALUES(?,?,'Observed container images',?,'system') ON CONFLICT(id) DO NOTHING`, versionID, artifactID, version); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_variants(id,artifact_version_id,digest,format,architecture)
			VALUES(?,?,?,'oci',?) ON CONFLICT(id) DO NOTHING`, variantID, versionID, digest, architecture); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return map[string]string{"artifactId": artifactID, "version": version, "variantId": variantID, "digest": digest}, nil
}

func (r *Repository) catalogUsedSIF(ctx context.Context, variant, digest string) (map[string]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var artifact, version, architecture string
	err = tx.QueryRowContext(ctx, `SELECT av.artifact_id,av.version,v.architecture
		FROM artifact_variants v JOIN artifact_versions av ON av.id=v.artifact_version_id
		WHERE v.id=? AND v.digest=? AND v.format='sif'`, variant, digest).Scan(&artifact, &version, &architecture)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	link := map[string]string{"artifactId": artifact, "version": version, "variantId": variant, "digest": digest}
	var reference string
	err = tx.QueryRowContext(ctx, `SELECT b.recipe_path FROM build_runs br
		JOIN artifact_builds b ON b.id=br.artifact_build_id
		WHERE br.output_variant_id=? AND br.status='completed' AND b.source_type='docker-image'
		ORDER BY br.finished_at DESC,br.id LIMIT 1`, variant).Scan(&reference)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if _, sourceDigest, pinned := strings.Cut(reference, "@"); pinned && executionImageDigest.MatchString(sourceDigest) {
		platform := ""
		if architecture != "" {
			platform = "linux/" + architecture
		}
		source, err := executionImageVariant(ctx, tx, sourceDigest, architecture, platform, "oci-manifest", false)
		if err != nil {
			return nil, err
		}
		link["sourceArtifactId"], link["sourceVersion"], link["sourceVariantId"] = source["artifactId"], source["version"], source["variantId"]
		link["sourceDigest"] = sourceDigest
	}
	return link, tx.Commit()
}

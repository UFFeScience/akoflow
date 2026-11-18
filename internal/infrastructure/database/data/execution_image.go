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

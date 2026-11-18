package environment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/google/uuid"
)

const connectionRevisionKey = "_akoflowConnectionRevision"

func decodeConnectionConfiguration(connection *domain.EnvironmentConnection, raw string) error {
	if err := json.Unmarshal([]byte(raw), &connection.Configuration); err != nil {
		return err
	}
	connection.Revision, _ = connection.Configuration[connectionRevisionKey].(string)
	delete(connection.Configuration, connectionRevisionKey)
	if connection.Revision == "" {
		connection.Revision = legacyConnectionRevision(*connection, raw)
	}
	return nil
}

func legacyConnectionRevision(connection domain.EnvironmentConnection, raw string) string {
	fields := []string{connection.ID, connection.EnvironmentID, connection.Name, string(connection.Type),
		connection.Endpoint, connection.Username, connection.CredentialRef, raw, connection.CreatedAt.UTC().Format(time.RFC3339Nano)}
	encoded, _ := json.Marshal(fields)
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func (r *Repository) UpdateConnectionRevision(ctx context.Context, updated domain.EnvironmentConnection) (domain.EnvironmentConnection, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.EnvironmentConnection{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE environment_connections SET configuration=configuration WHERE id=? AND environment_id=?`, updated.ID, updated.EnvironmentID); err != nil {
		return domain.EnvironmentConnection{}, err
	}
	current, err := readConnectionRevision(ctx, tx, updated.ID, updated.EnvironmentID)
	if err != nil {
		return domain.EnvironmentConnection{}, err
	}
	if updated.Revision == "" || updated.Revision != current.Revision {
		return domain.EnvironmentConnection{}, ports.ErrConnectionRevisionConflict
	}
	updated.CreatedAt = current.CreatedAt
	updated.Revision = uuid.NewString()
	configuration := maps.Clone(updated.Configuration)
	if configuration == nil {
		configuration = make(map[string]any)
	}
	delete(updated.Configuration, connectionRevisionKey)
	configuration[connectionRevisionKey] = updated.Revision
	encoded, err := json.Marshal(configuration)
	if err != nil {
		return domain.EnvironmentConnection{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE environment_connections SET
		name=?,type=?,endpoint=?,username=?,credential_ref=?,configuration=?
		WHERE id=? AND environment_id=?`, updated.Name, updated.Type, updated.Endpoint,
		updated.Username, updated.CredentialRef, string(encoded), updated.ID, updated.EnvironmentID); err != nil {
		return domain.EnvironmentConnection{}, err
	}
	return updated, tx.Commit()
}

func readConnectionRevision(ctx context.Context, tx *sql.Tx, id, environmentID string) (domain.EnvironmentConnection, error) {
	var current domain.EnvironmentConnection
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT id,environment_id,name,type,endpoint,username,credential_ref,configuration,created_at
		FROM environment_connections WHERE id=? AND environment_id=?`, id, environmentID).
		Scan(&current.ID, &current.EnvironmentID, &current.Name, &current.Type, &current.Endpoint,
			&current.Username, &current.CredentialRef, &raw, &current.CreatedAt)
	if err == sql.ErrNoRows {
		return current, ports.ErrConnectionRevisionConflict
	}
	if err != nil {
		return current, err
	}
	err = decodeConnectionConfiguration(&current, raw)
	return current, err
}

func encodeNewConnectionRevision(values map[string]any) ([]byte, error) {
	configuration := maps.Clone(values)
	if configuration == nil {
		configuration = make(map[string]any)
	}
	configuration[connectionRevisionKey] = uuid.NewString()
	return json.Marshal(configuration)
}

func verifyConnectionRevisions(ctx context.Context, tx *sql.Tx, definition domain.EnvironmentDefinition) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM environment_connections WHERE environment_id=?`, definition.Environment.ID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(ids) != len(definition.ConnectionRevisions) {
		return ports.ErrConnectionRevisionConflict
	}
	for _, id := range ids {
		connection, err := readConnectionRevision(ctx, tx, id, definition.Environment.ID)
		if err != nil {
			return err
		}
		if definition.ConnectionRevisions[id] != connection.Revision {
			return ports.ErrConnectionRevisionConflict
		}
	}
	return nil
}

package environment

import (
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) ExecutionDefinition(ctx context.Context, versionID string) (*domain.EnvironmentDefinition, error) {
	definition := &domain.EnvironmentDefinition{}
	err := r.db.QueryRowContext(ctx, "SELECT id FROM environment_versions WHERE id=?", versionID).Scan(&definition.Version.ID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	definition.Resources, err = r.listResources(ctx, versionID)
	if err != nil {
		return nil, err
	}
	definition.Runtimes, err = r.listRuntimes(ctx, versionID)
	if err != nil {
		return nil, err
	}
	definition.RuntimeBindings, err = r.listRuntimeBindings(ctx, versionID)
	if err != nil {
		return nil, err
	}
	definition.Profiles, err = r.listProfiles(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return definition, nil
}

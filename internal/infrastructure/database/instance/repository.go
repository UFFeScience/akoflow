package instance

import (
	"context"
	"database/sql"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (repository *Repository) Find(ctx context.Context) (*domaininstance.Instance, error) {
	const query = `SELECT id, name, description, organization, location, created_at, updated_at
		FROM system_instance LIMIT 1`
	value := &domaininstance.Instance{}
	err := repository.db.QueryRowContext(ctx, query).Scan(
		&value.ID,
		&value.Name,
		&value.Description,
		&value.Organization,
		&value.Location,
		&value.CreatedAt,
		&value.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

func (repository *Repository) Save(ctx context.Context, value domaininstance.Instance) error {
	const query = `INSERT INTO system_instance (
		id, name, description, organization, location, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		description = excluded.description,
		organization = excluded.organization,
		location = excluded.location,
		updated_at = CURRENT_TIMESTAMP`
	_, err := repository.db.ExecContext(
		ctx,
		query,
		value.ID,
		value.Name,
		value.Description,
		value.Organization,
		value.Location,
	)
	return err
}

func (repository *Repository) FindPreferences(ctx context.Context, clientID string) (*domaininstance.UserPreferences, error) {
	value := &domaininstance.UserPreferences{}
	err := repository.db.QueryRowContext(ctx, `SELECT client_id, theme, animations_enabled, updated_at
		FROM user_preferences WHERE client_id=?`, clientID).Scan(
		&value.ClientID, &value.Theme, &value.AnimationsEnabled, &value.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

func (repository *Repository) SavePreferences(ctx context.Context, value domaininstance.UserPreferences) error {
	_, err := repository.db.ExecContext(ctx, `INSERT INTO user_preferences (
		client_id, theme, animations_enabled, updated_at
	) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(client_id) DO UPDATE SET
		theme=excluded.theme,
		animations_enabled=excluded.animations_enabled,
		updated_at=CURRENT_TIMESTAMP`, value.ClientID, value.Theme, value.AnimationsEnabled)
	return err
}

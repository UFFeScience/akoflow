package environment

import (
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/application/installationhealth"
)

func (r *Repository) InstallationConnectionHealth(ctx context.Context) ([]installationhealth.Observation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.environment_id,last.status,last.checked_at
		FROM environment_connections c
		LEFT JOIN environment_connection_checks last ON last.id=(
			SELECT checks.id FROM environment_connection_checks checks
			WHERE checks.connection_id=c.id ORDER BY checks.checked_at DESC,checks.id DESC LIMIT 1
		) ORDER BY c.environment_id,c.name,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []installationhealth.Observation{}
	for rows.Next() {
		var item installationhealth.Observation
		var status sql.NullString
		var checkedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.EnvironmentID, &status, &checkedAt); err != nil {
			return nil, err
		}
		if checkedAt.Valid {
			item.ObservedAt = &checkedAt.Time
		}
		switch status.String {
		case "online":
			item.Status, item.Detail = "healthy", "The last connection check succeeded."
		case "offline":
			item.Status, item.Detail = "unavailable", "The last connection check failed."
			item.NextAction = "Check this connection in its environment."
		default:
			item.Status, item.Detail = "unknown", "No connection check has been recorded."
			item.NextAction = "Run a connection check from its environment."
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

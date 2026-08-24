package workflow

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Definition = domain.WorkflowDefinition
type Repository struct{ db *sql.DB }

var _ ports.WorkflowStore = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context) ([]domain.WorkflowDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM workflow_definitions ORDER BY namespace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	definitions := make([]domain.WorkflowDefinition, 0, len(ids))
	for _, id := range ids {
		definition, err := r.Find(ctx, id)
		if err != nil {
			return nil, err
		}
		if definition != nil {
			definitions = append(definitions, *definition)
		}
	}
	return definitions, nil
}

func (r *Repository) Find(ctx context.Context, id string) (*domain.WorkflowDefinition, error) {
	var definition domain.WorkflowDefinition
	err := r.db.QueryRowContext(ctx, `SELECT id, external_id, name, namespace
		FROM workflow_definitions WHERE id=?`, id).Scan(&definition.ID,
		&definition.ExternalID, &definition.Name, &definition.Namespace)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var versionID string
	err = r.db.QueryRowContext(ctx, `SELECT id FROM workflow_versions
		WHERE workflow_id=? ORDER BY version DESC LIMIT 1`, id).Scan(&versionID)
	if err != nil {
		return nil, err
	}
	version, err := r.FindVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if version != nil {
		definition.Version = *version
	}
	types, err := r.listTypes(ctx, versionID)
	if err != nil {
		return nil, err
	}
	definition.Types = types
	return &definition, nil
}

func (r *Repository) listTypes(ctx context.Context, versionID string) ([]domain.ActivityType, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT t.id, t.name, t.application,
		t.default_image, t.cpu_intensity, t.memory_intensity, t.io_intensity,
		t.network_intensity, t.metadata FROM activity_types t
		JOIN activity_definitions a ON a.activity_type_id=t.id
		WHERE a.workflow_version_id=? ORDER BY t.name`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	types := make([]domain.ActivityType, 0)
	for rows.Next() {
		var item domain.ActivityType
		var metadata string
		if err := rows.Scan(&item.ID, &item.Name, &item.Application, &item.DefaultImage,
			&item.CPUIntensity, &item.MemoryIntensity, &item.IOIntensity,
			&item.NetworkIntensity, &metadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadata), &item.Metadata); err != nil {
			return nil, err
		}
		types = append(types, item)
	}
	return types, rows.Err()
}

func (r *Repository) Create(ctx context.Context, definition Definition) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO workflow_definitions (id, external_id, name, namespace)
		VALUES (?, ?, ?, ?)`, definition.ID, definition.ExternalID, definition.Name,
		definition.Namespace); err != nil {
		return err
	}
	version := definition.Version
	if _, err = tx.Exec(`INSERT INTO workflow_versions (
		id, workflow_id, version, definition_hash, status
	) VALUES (?, ?, ?, ?, 'published')`, version.ID, definition.ID, version.Version,
		version.DefinitionHash); err != nil {
		return err
	}
	for _, activityType := range definition.Types {
		metadata, _ := json.Marshal(activityType.Metadata)
		if _, err = tx.Exec(`INSERT OR IGNORE INTO activity_types (
			id, name, application, default_image, cpu_intensity, memory_intensity,
			io_intensity, network_intensity, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, activityType.ID, activityType.Name,
			activityType.Application, activityType.DefaultImage, activityType.CPUIntensity,
			activityType.MemoryIntensity, activityType.IOIntensity,
			activityType.NetworkIntensity, string(metadata)); err != nil {
			return err
		}
	}
	for _, activity := range version.Activities {
		if err = activity.Validate(); err != nil {
			return err
		}
		metadata, _ := json.Marshal(activity.Metadata)
		capabilities, _ := json.Marshal(activity.Capabilities)
		command, _ := json.Marshal(activity.Command)
		resources, _ := json.Marshal(activity.Resources)
		service, _ := json.Marshal(activity.Service)
		simulation, _ := json.Marshal(activity.Simulation)
		policy, _ := json.Marshal(activity.Policy)
		if _, err = tx.Exec(`INSERT INTO activity_definitions (
			id, workflow_version_id, activity_type_id, external_id, name, kind,
			capabilities, command_spec, resource_requirements, service_spec,
			simulation_spec, policy, priority, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, activity.ID, version.ID,
			activity.ActivityTypeID, activity.ExternalID, activity.Name, activity.Kind,
			string(capabilities), string(command), string(resources), nullableJSON(activity.Service, service),
			nullableJSON(activity.Simulation, simulation), string(policy), activity.Priority,
			string(metadata)); err != nil {
			return err
		}
	}
	for _, dependency := range version.Dependencies {
		if _, err = tx.Exec(`INSERT INTO activity_dependencies (
			activity_id, depends_on_activity_id, dependency_type
		) VALUES (?, ?, ?)`, dependency.ActivityID, dependency.DependsOnActivityID,
			dependency.Type); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) FindVersion(ctx context.Context, id string) (*domain.WorkflowVersion, error) {
	var workflow domain.WorkflowVersion
	err := r.db.QueryRowContext(ctx, `SELECT id, workflow_id, version, definition_hash
		FROM workflow_versions WHERE id=?`, id).Scan(&workflow.ID, &workflow.WorkflowID,
		&workflow.Version, &workflow.DefinitionHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, workflow_version_id, activity_type_id,
		external_id, name, kind, capabilities, command_spec, resource_requirements,
		service_spec, simulation_spec, policy, priority, metadata
		FROM activity_definitions WHERE workflow_version_id=?`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var activity domain.Activity
		var capabilities, command, resources, policy, metadata string
		var service, simulation sql.NullString
		if err := rows.Scan(&activity.ID, &activity.WorkflowVersionID,
			&activity.ActivityTypeID, &activity.ExternalID, &activity.Name,
			&activity.Kind, &capabilities, &command, &resources, &service,
			&simulation, &policy, &activity.Priority, &metadata); err != nil {
			rows.Close()
			return nil, err
		}
		_ = json.Unmarshal([]byte(metadata), &activity.Metadata)
		_ = json.Unmarshal([]byte(capabilities), &activity.Capabilities)
		_ = json.Unmarshal([]byte(command), &activity.Command)
		_ = json.Unmarshal([]byte(resources), &activity.Resources)
		_ = json.Unmarshal([]byte(policy), &activity.Policy)
		if service.Valid {
			activity.Service = &domain.ServiceSpec{}
			_ = json.Unmarshal([]byte(service.String), activity.Service)
		}
		if simulation.Valid {
			activity.Simulation = &domain.ActivitySimulation{}
			_ = json.Unmarshal([]byte(simulation.String), activity.Simulation)
		}
		workflow.Activities = append(workflow.Activities, activity)
	}
	rows.Close()
	deps, err := r.db.QueryContext(ctx, `SELECT d.activity_id, d.depends_on_activity_id,
		d.dependency_type FROM activity_dependencies d
		JOIN activity_definitions a ON a.id=d.activity_id
		WHERE a.workflow_version_id=?`, id)
	if err != nil {
		return nil, err
	}
	defer deps.Close()
	for deps.Next() {
		var dependency domain.ActivityDependency
		if err := deps.Scan(&dependency.ActivityID, &dependency.DependsOnActivityID,
			&dependency.Type); err != nil {
			return nil, err
		}
		workflow.Dependencies = append(workflow.Dependencies, dependency)
	}
	return &workflow, deps.Err()
}

func nullableJSON(value any, encoded []byte) any {
	if value == nil {
		return nil
	}
	return string(encoded)
}

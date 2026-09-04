package cloud

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	applicationconfiguration "github.com/UFFeScience/akoflow/internal/application/machineconfiguration"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

const defaultPlaybook = `---
- name: Configure Akoflow scientific worker
  hosts: all
  become: true
  tasks:
    - name: Install base packages
      ansible.builtin.apt:
        name:
          - ca-certificates
          - curl
          - rsync
          - tar
        state: present
    - name: Install Docker
      ansible.builtin.apt:
        name: docker.io
        state: present
    - name: Enable Docker
      ansible.builtin.service:
        name: docker
        state: started
        enabled: true
    - name: Allow the remote user to run Docker
      ansible.builtin.user:
        name: "{{ ansible_user_id }}"
        groups: docker
        append: true
    - name: Install Apptainer
      ansible.builtin.apt:
        name: apptainer
        state: present
    - name: Create Akoflow workspace
      ansible.builtin.file:
        path: "{{ akoflow_workspace_path }}"
        state: directory
        owner: "{{ ansible_user_id }}"
        mode: "0770"
`

func (r *Repository) EnsureDefaults(ctx context.Context) error {
	validation := applicationconfiguration.ValidatePlaybook(defaultPlaybook)
	if !validation.Valid {
		return fmt.Errorf("invalid built-in machine configuration: %v", validation.Errors)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO machine_configurations
		(id,name,description,ownership,enabled,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
		domain.DefaultMachineConfigurationID, "Akoflow Scientific Worker", "Installs and validates Docker, Apptainer and the remote scientific workspace.", "system", true, now, now); err != nil {
		return err
	}
	compatibility := domain.MachineConfigurationCompatibility{Providers: []string{"gcp", "aws", "azure"}, OperatingSystems: []string{"ubuntu-22.04", "ubuntu-24.04"}, Architectures: []string{"amd64"}}
	checks := []domain.MachineConfigurationValidationCheck{
		{Name: "Docker available", Command: "docker version", Capability: "docker"},
		{Name: "Apptainer available", Command: "apptainer version", Capability: "apptainer"},
		{Name: "Workspace writable", Command: `test -w "${AKOFLOW_WORKSPACE:-/akoflow/workspace}"`, Capability: "workspace"},
	}
	compatJSON, _ := json.Marshal(compatibility)
	variablesJSON := []byte(`{"type":"object","properties":{"akoflow_workspace_path":{"type":"string","default":"/akoflow/workspace"}},"additionalProperties":false}`)
	checksJSON, _ := json.Marshal(checks)
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO machine_configuration_versions
		(id,machine_configuration_id,version,status,playbook_yaml,content_sha256,compatibility,variables_schema,validation_checks,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, domain.DefaultMachineConfigurationVersionID,
		domain.DefaultMachineConfigurationID, 2, "published", defaultPlaybook,
		validation.SHA256, compatJSON, variablesJSON, checksJSON, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CreateMachineConfiguration(ctx context.Context, value domain.MachineConfiguration) error {
	if value.ID == "" || value.Name == "" {
		return fmt.Errorf("machine configuration id and name are required")
	}
	if value.Ownership == "" {
		value.Ownership = "user"
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = value.CreatedAt
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO machine_configurations(
		id,name,description,ownership,enabled,created_at,updated_at
	) VALUES(?,?,?,?,?,?,?)`, value.ID, value.Name, value.Description, value.Ownership,
		value.Enabled, value.CreatedAt, value.UpdatedAt)
	return err
}

func (r *Repository) CreateMachineConfigurationVersion(ctx context.Context, value domain.MachineConfigurationVersion) error {
	validation := applicationconfiguration.ValidatePlaybook(value.PlaybookYAML)
	if !validation.Valid {
		return fmt.Errorf("invalid playbook: %v", validation.Errors)
	}
	if value.ID == "" || value.MachineConfigurationID == "" || value.Version < 1 {
		return fmt.Errorf("version id, configuration id and positive version are required")
	}
	if value.Status == "" {
		value.Status = "draft"
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	compatibility, _ := json.Marshal(value.Compatibility)
	variables, _ := json.Marshal(value.VariablesSchema)
	checks, _ := json.Marshal(value.ValidationChecks)
	_, err := r.db.ExecContext(ctx, `INSERT INTO machine_configuration_versions(
		id,machine_configuration_id,version,status,playbook_yaml,content_sha256,
		compatibility,variables_schema,validation_checks,created_at
	) VALUES(?,?,?,?,?,?,?,?,?,?)`, value.ID, value.MachineConfigurationID, value.Version,
		value.Status, value.PlaybookYAML, validation.SHA256, compatibility, variables, checks,
		value.CreatedAt)
	return err
}

func (r *Repository) ListMachineConfigurations(ctx context.Context) ([]domain.MachineConfiguration, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM machine_configurations ORDER BY ownership,name`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	var values []domain.MachineConfiguration
	for _, id := range ids {
		value, err := r.FindMachineConfiguration(ctx, id)
		if err != nil {
			return nil, err
		}
		if value != nil {
			values = append(values, *value)
		}
	}
	return values, nil
}

func (r *Repository) FindMachineConfiguration(ctx context.Context, id string) (*domain.MachineConfiguration, error) {
	var value domain.MachineConfiguration
	err := r.db.QueryRowContext(ctx, `SELECT id,name,description,ownership,enabled,created_at,updated_at
		FROM machine_configurations WHERE id=?`, id).Scan(&value.ID, &value.Name,
		&value.Description, &value.Ownership, &value.Enabled, &value.CreatedAt, &value.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,machine_configuration_id,version,status,
		playbook_yaml,content_sha256,compatibility,variables_schema,validation_checks,created_at
		FROM machine_configuration_versions WHERE machine_configuration_id=? ORDER BY version DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.MachineConfigurationVersion
		var compatibility, variables, checks []byte
		if err := rows.Scan(&v.ID, &v.MachineConfigurationID, &v.Version, &v.Status, &v.PlaybookYAML, &v.ContentSHA256, &compatibility, &variables, &checks, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(compatibility, &v.Compatibility)
		_ = json.Unmarshal(variables, &v.VariablesSchema)
		_ = json.Unmarshal(checks, &v.ValidationChecks)
		value.Versions = append(value.Versions, v)
	}
	return &value, rows.Err()
}

func (r *Repository) FindMachineConfigurationVersion(
	ctx context.Context,
	id string,
) (*domain.MachineConfigurationVersion, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,machine_configuration_id,version,status,
		playbook_yaml,content_sha256,compatibility,variables_schema,validation_checks,created_at
		FROM machine_configuration_versions WHERE id=?`, id)
	var value domain.MachineConfigurationVersion
	var compatibility, variables, checks []byte
	err := row.Scan(
		&value.ID, &value.MachineConfigurationID, &value.Version, &value.Status,
		&value.PlaybookYAML, &value.ContentSHA256, &compatibility, &variables,
		&checks, &value.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(compatibility, &value.Compatibility)
	_ = json.Unmarshal(variables, &value.VariablesSchema)
	_ = json.Unmarshal(checks, &value.ValidationChecks)
	return &value, nil
}

func (r *Repository) CreateCapacityTarget(ctx context.Context, value domain.CloudCapacityTarget) error {
	if value.ID == "" || value.EnvironmentID == "" || value.Name == "" || value.ProviderMachineType == "" || value.ImageReference == "" {
		return fmt.Errorf("target id, environment, name, machine type and image are required")
	}
	if value.ZonePolicy == "" {
		value.ZonePolicy = "any-compatible"
	}
	if value.Architecture == "" {
		value.Architecture = "amd64"
	}
	if value.ProvisioningMode == "" {
		value.ProvisioningMode = "standard"
	}
	if value.MaximumInstances < 1 {
		value.MaximumInstances = 1
	}
	if value.LifecyclePolicy == "" {
		value.LifecyclePolicy = "destroy-after-execution"
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	configuration, _ := json.Marshal(value.Configuration)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO cloud_capacity_targets(
		id,environment_id,name,provider,provider_machine_type,region,zone_policy,fixed_zone,
		image_reference,architecture,vcpu,memory_mib,provisioning_mode,maximum_instances,
		lifecycle_policy,configuration,enabled,created_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.EnvironmentID, value.Name,
		value.Provider, value.ProviderMachineType, value.Region, value.ZonePolicy, value.FixedZone,
		value.ImageReference, value.Architecture, value.VCPU, value.MemoryMiB,
		value.ProvisioningMode, value.MaximumInstances, value.LifecyclePolicy, configuration,
		value.Enabled, value.CreatedAt)
	if err != nil {
		return err
	}
	configurations := value.MachineConfigurations
	hasDefault := false
	for _, configuration := range configurations {
		if configuration.ConfigurationVersionID == domain.DefaultMachineConfigurationVersionID {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		configurations = append([]domain.CloudTargetConfiguration{{
			ConfigurationVersionID: domain.DefaultMachineConfigurationVersionID,
			ExecutionOrder:         0, Required: true, Enabled: true,
		}}, configurations...)
	}
	for _, item := range configurations {
		variables, _ := json.Marshal(item.Variables)
		_, err = tx.ExecContext(ctx, `INSERT INTO cloud_capacity_target_configurations(
			capacity_target_id,configuration_version_id,execution_order,variables,required,enabled
		) VALUES(?,?,?,?,?,?)`, value.ID, item.ConfigurationVersionID, item.ExecutionOrder,
			variables, item.Required, item.Enabled)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) ListCapacityTargets(ctx context.Context, environmentID string) ([]domain.CloudCapacityTarget, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,environment_id,name,provider,provider_machine_type,
		region,zone_policy,fixed_zone,image_reference,architecture,vcpu,memory_mib,
		provisioning_mode,maximum_instances,lifecycle_policy,configuration,enabled,created_at
		FROM cloud_capacity_targets WHERE environment_id=? ORDER BY name`, environmentID)
	if err != nil {
		return nil, err
	}
	var values []domain.CloudCapacityTarget
	for rows.Next() {
		var v domain.CloudCapacityTarget
		var configuration []byte
		if err := rows.Scan(
			&v.ID, &v.EnvironmentID, &v.Name, &v.Provider, &v.ProviderMachineType,
			&v.Region, &v.ZonePolicy, &v.FixedZone, &v.ImageReference, &v.Architecture,
			&v.VCPU, &v.MemoryMiB, &v.ProvisioningMode, &v.MaximumInstances,
			&v.LifecyclePolicy, &configuration, &v.Enabled, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(configuration, &v.Configuration)
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range values {
		cr, err := r.db.QueryContext(ctx, `SELECT configuration_version_id,execution_order,
			variables,required,enabled FROM cloud_capacity_target_configurations
			WHERE capacity_target_id=? ORDER BY execution_order`, values[index].ID)
		if err != nil {
			return nil, err
		}
		for cr.Next() {
			var c domain.CloudTargetConfiguration
			var variables []byte
			if err := cr.Scan(&c.ConfigurationVersionID, &c.ExecutionOrder, &variables, &c.Required, &c.Enabled); err != nil {
				cr.Close()
				return nil, err
			}
			_ = json.Unmarshal(variables, &c.Variables)
			values[index].MachineConfigurations = append(values[index].MachineConfigurations, c)
		}
		if err := cr.Err(); err != nil {
			cr.Close()
			return nil, err
		}
		cr.Close()
	}
	return values, nil
}

func (r *Repository) FindCapacityTarget(ctx context.Context, id string) (*domain.CloudCapacityTarget, error) {
	var environmentID string
	err := r.db.QueryRowContext(ctx, `SELECT environment_id FROM cloud_capacity_targets WHERE id=?`, id).Scan(&environmentID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	values, err := r.ListCapacityTargets(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	for index := range values {
		if values[index].ID == id {
			return &values[index], nil
		}
	}
	return nil, nil
}

func (r *Repository) CreateProvisionedInstance(ctx context.Context, value domain.CloudProvisionedInstance) error {
	disk, _ := json.Marshal(value.Disk)
	output, _ := json.Marshal(value.TerraformOutput)
	_, err := r.db.ExecContext(ctx, `INSERT INTO cloud_provisioned_instances(
		id,capacity_target_id,environment_id,provider,provider_id,name,status,public_address,
		private_address,ssh_username,ssh_credential_ref,disk,terraform_output,failure_reason,
		created_at,ready_at,destroyed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		value.ID, value.CapacityTargetID, value.EnvironmentID, value.Provider, value.ProviderID,
		value.Name, value.Status, value.PublicAddress, value.PrivateAddress, value.SSHUsername,
		value.SSHCredentialRef, disk, output, value.FailureReason, value.CreatedAt, value.ReadyAt,
		value.DestroyedAt)
	return err
}

func (r *Repository) UpdateProvisionedInstance(ctx context.Context, value domain.CloudProvisionedInstance) error {
	disk, _ := json.Marshal(value.Disk)
	output, _ := json.Marshal(value.TerraformOutput)
	_, err := r.db.ExecContext(ctx, `UPDATE cloud_provisioned_instances SET provider_id=?,name=?,status=?,
		public_address=?,private_address=?,ssh_username=?,ssh_credential_ref=?,disk=?,
		terraform_output=?,failure_reason=?,ready_at=?,destroyed_at=? WHERE id=?`,
		value.ProviderID, value.Name, value.Status, value.PublicAddress, value.PrivateAddress,
		value.SSHUsername, value.SSHCredentialRef, disk, output, value.FailureReason,
		value.ReadyAt, value.DestroyedAt, value.ID)
	return err
}

func (r *Repository) FindProvisionedInstance(ctx context.Context, id string) (*domain.CloudProvisionedInstance, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,capacity_target_id,environment_id,provider,provider_id,
		name,status,public_address,private_address,ssh_username,ssh_credential_ref,disk,
		terraform_output,failure_reason,created_at,ready_at,destroyed_at
		FROM cloud_provisioned_instances WHERE id=?`, id)
	value, err := scanProvisionedInstance(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

func (r *Repository) ListProvisionedInstances(ctx context.Context, environmentID string) ([]domain.CloudProvisionedInstance, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,capacity_target_id,environment_id,provider,provider_id,
		name,status,public_address,private_address,ssh_username,ssh_credential_ref,disk,
		terraform_output,failure_reason,created_at,ready_at,destroyed_at
		FROM cloud_provisioned_instances WHERE environment_id=? ORDER BY created_at DESC`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]domain.CloudProvisionedInstance, 0)
	for rows.Next() {
		value, scanErr := scanProvisionedInstance(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, *value)
	}
	return values, rows.Err()
}

type scanner func(...any) error

func scanProvisionedInstance(scan scanner) (*domain.CloudProvisionedInstance, error) {
	var value domain.CloudProvisionedInstance
	var disk, output []byte
	err := scan(
		&value.ID, &value.CapacityTargetID, &value.EnvironmentID, &value.Provider,
		&value.ProviderID, &value.Name, &value.Status, &value.PublicAddress,
		&value.PrivateAddress, &value.SSHUsername, &value.SSHCredentialRef, &disk,
		&output, &value.FailureReason, &value.CreatedAt, &value.ReadyAt, &value.DestroyedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(disk, &value.Disk)
	_ = json.Unmarshal(output, &value.TerraformOutput)
	return &value, nil
}

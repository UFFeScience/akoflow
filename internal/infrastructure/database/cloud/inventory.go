package cloud

import (
	"context"
	"encoding/json"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Inventory excludes configuration recipes, Terraform output, logs and billing
// history. Resource rows need only target matching, readiness and connectivity.
func (r *Repository) Inventory(ctx context.Context) (domain.CloudInventory, error) {
	var inventory domain.CloudInventory
	var targets, instances string
	err := r.db.QueryRowContext(ctx, `SELECT
		(SELECT json_group_array(json_object('id',id,'environmentId',environment_id,
			'name',name,'vcpu',vcpu,'memoryMiB',memory_mib)) FROM cloud_capacity_targets WHERE enabled=1),
		(SELECT json_group_array(json_object('id',id,'environmentId',environment_id,
			'capacityTargetId',capacity_target_id,'status',status,'publicAddress',public_address,
			'privateAddress',private_address)) FROM cloud_provisioned_instances)`).Scan(&targets, &instances)
	if err != nil {
		return inventory, err
	}
	if err := json.Unmarshal([]byte(targets), &inventory.Targets); err != nil {
		return inventory, err
	}
	if err := json.Unmarshal([]byte(instances), &inventory.Instances); err != nil {
		return inventory, err
	}
	return inventory, nil
}

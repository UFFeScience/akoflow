package workflow_engine_api_handler

import (
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
	"math"
)

// VCPU remains the physical VM size. A smaller scheduling capacity deliberately
// limits concurrent execution lanes without changing the provider machine type.
func targetSchedulingCapacity(target domain.CloudCapacityTarget) (float64, error) {
	if _, exists := target.Configuration["schedulingCpuCapacity"]; !exists {
		return float64(target.VCPU), nil
	}
	value := configurationFloat64(target.Configuration, "schedulingCpuCapacity")
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > float64(target.VCPU) {
		return 0, fmt.Errorf("schedulingCpuCapacity must be positive and no larger than target vcpu")
	}
	return value, nil
}

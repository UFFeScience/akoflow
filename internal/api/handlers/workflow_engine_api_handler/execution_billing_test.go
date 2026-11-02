package workflow_engine_api_handler

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
	"time"
)

func TestLifecycleBillingCountsSharedGenerationOnce(t *testing.T) {
	start := time.Unix(100, 0)
	end := start.Add(time.Hour)
	billing := &domain.InstanceBilling{StartedAt: start, ComputeStartedAt: &start, ComputePricePerSecond: 1.0 / 3600, DiskPricePerSecond: 0.1 / 3600, PricingStatus: "estimated", PriceCurrency: "USD"}
	row := executionInstanceBilling{InstanceID: "vm-1", Billing: billing}
	result := sumExecutionCloudBilling(executionCloudBilling{Status: "estimated", AsOf: end, Instances: []executionInstanceBilling{row, row}})
	if result.EstimatedCost == nil || *result.EstimatedCost != 1.1 {
		t.Fatalf("double counted VM: %+v", result)
	}
}

func TestMissingLifecycleTariffIsNotFree(t *testing.T) {
	result := sumExecutionCloudBilling(executionCloudBilling{Status: "estimated", AsOf: time.Now(), Instances: []executionInstanceBilling{{InstanceID: "vm-1"}}})
	if result.Status != "incomplete" || result.EstimatedCost != nil {
		t.Fatalf("missing tariff became free: %+v", result)
	}
}

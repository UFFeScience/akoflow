package workflow_engine_api_handler

import (
	"context"
	"sort"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// These are lifecycle estimates, not activity charges or a provider invoice.
// One concrete generation contributes once, even when many tasks share it.
type executionCloudBilling struct {
	CostBasis     string                     `json:"costBasis"`
	ExcludedCosts []string                   `json:"excludedCosts"`
	Status        string                     `json:"status"`
	Currency      string                     `json:"currency,omitempty"`
	EstimatedCost *float64                   `json:"estimatedCost"`
	KnownCost     float64                    `json:"knownCost"`
	Instances     []executionInstanceBilling `json:"instances"`
	AsOf          time.Time                  `json:"asOf"`
}
type executionInstanceBilling struct {
	InstanceID string                  `json:"instanceId"`
	Status     string                  `json:"status"`
	Billing    *domain.InstanceBilling `json:"billing,omitempty"`
}

func (h *Handler) executionCloudBilling(ctx context.Context, tasks []domain.TaskExecution, operations []domain.CloudOperationRun) (executionCloudBilling, error) {
	ids := map[string]bool{}
	for _, task := range tasks {
		if task.CloudInstanceID != "" {
			ids[task.CloudInstanceID] = true
		}
	}
	for _, operation := range operations {
		if operation.InstanceID != "" {
			ids[operation.InstanceID] = true
		}
	}
	result := executionCloudBilling{Status: "estimated", AsOf: time.Now().UTC(), Instances: []executionInstanceBilling{}, CostBasis: "VM compute and boot-disk lifecycle at on-demand marginal rates; not a provider invoice", ExcludedCosts: []string{"network-egress", "public-ipv4", "image-storage", "discounts-and-tax"}}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		row := executionInstanceBilling{InstanceID: id, Status: "missing"}
		if h.cloud != nil {
			instance, err := h.cloud.FindProvisionedInstance(ctx, id)
			if err != nil {
				return result, err
			}
			if instance != nil {
				row.Status, row.Billing = instance.Status, instance.Billing
			}
		}
		result.Instances = append(result.Instances, row)
	}
	return sumExecutionCloudBilling(result), nil
}

func sumExecutionCloudBilling(result executionCloudBilling) executionCloudBilling {
	seen := map[string]bool{}
	for _, row := range result.Instances {
		if seen[row.InstanceID] {
			continue
		}
		seen[row.InstanceID] = true
		billing := row.Billing
		if billing == nil {
			result.Status = "incomplete"
			continue
		}
		billing.RefreshCost(result.AsOf)
		if billing.PricingStatus != "estimated" || billing.PriceCurrency == "" {
			result.Status = "incomplete"
		}
		if result.Currency == "" {
			result.Currency = billing.PriceCurrency
		}
		if billing.PriceCurrency != result.Currency {
			result.Status = "incomplete"
			continue
		}
		result.KnownCost += billing.CurrentCost
	}
	if result.Status == "estimated" {
		total := result.KnownCost
		result.EstimatedCost = &total
	}
	return result
}

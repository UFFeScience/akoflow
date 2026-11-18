package planning

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/resource"
)

type planPricing struct {
	Scope    string                        `json:"scope"`
	Complete bool                          `json:"complete"`
	Compute  map[string]resource.UnitPrice `json:"compute"`
	Disk     map[string]resource.UnitPrice `json:"disk,omitempty"`
	Network  []resource.UnitPrice          `json:"network,omitempty"`
}

func recordPlanPricing(plan *domain.SchedulePlan, request domain.PlanningRequest) {
	byID := make(map[string]domain.Resource, len(request.Resources))
	for _, item := range request.Resources {
		byID[item.ID] = item
	}
	scope, _ := plan.Metadata["costModel"].(string)
	if scope == "" {
		scope = "task-runtime-and-network"
	}
	pricing := planPricing{
		Scope: scope, Complete: len(plan.Assignments) > 0,
		Compute: map[string]resource.UnitPrice{}, Disk: map[string]resource.UnitPrice{},
	}
	transfers := false
	for _, assignment := range plan.Assignments {
		item := byID[assignment.ResourceID]
		price := item.ComputePrice()
		pricing.Compute[assignment.ResourceID] = price
		pricing.Complete = pricing.Complete && price.Rate != nil
		transfers = transfers || assignment.PredictedTransferSeconds > 0
		if _, modeled := plan.Metadata["cloudCost."+item.ID]; modeled && scope != "moheft-task-additive" {
			pricing.Scope = "planned-cloud-lifecycle"
			if item.StorageBytes > 0 {
				disk := item.DiskPrice()
				pricing.Disk[item.ID] = disk
				pricing.Complete = pricing.Complete && disk.Rate != nil
			}
		}
	}
	if transfers {
		pricing.Complete = pricing.Complete && len(request.NetworkTopology.Links) > 0
		for _, link := range request.NetworkTopology.Links {
			price := link.TransferPrice()
			pricing.Network = append(pricing.Network, price)
			pricing.Complete = pricing.Complete && price.Rate != nil
		}
	}
	if plan.Metadata == nil {
		plan.Metadata = map[string]any{}
	}
	plan.Metadata["pricing"] = pricing
}

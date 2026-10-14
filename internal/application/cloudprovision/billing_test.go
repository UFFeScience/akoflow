package cloudprovision

import (
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestInstanceBillingPauseAndResume(t *testing.T) {
	start := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	instance := domain.CloudProvisionedInstance{
		Billing: billingForTarget(domain.CloudCapacityTarget{Configuration: map[string]any{
			"pricePerHour":         float64(3.6),
			"diskSizeGiB":          float64(10),
			"diskPricePerGiBMonth": float64(7.3),
		}}, start),
	}
	if instance.Billing.ComputePricePerSecond != 0.001 {
		t.Fatalf("compute rate = %v", instance.Billing.ComputePricePerSecond)
	}
	if instance.Billing.DiskPricePerSecond <= 0 {
		t.Fatal("disk rate was not captured")
	}
	pauseBilling(&instance, start.Add(10*time.Second))
	pauseBilling(&instance, start.Add(20*time.Second))
	if instance.Billing.AccumulatedComputeSeconds != 10 {
		t.Fatalf("compute seconds after stop = %v", instance.Billing.AccumulatedComputeSeconds)
	}
	resumeBilling(&instance, start.Add(30*time.Second))
	resumeBilling(&instance, start.Add(31*time.Second))
	pauseBilling(&instance, start.Add(35*time.Second))
	if instance.Billing.AccumulatedComputeSeconds != 15 {
		t.Fatalf("compute seconds after restart = %v", instance.Billing.AccumulatedComputeSeconds)
	}
	instance.Billing.RefreshCost(start.Add(35 * time.Second))
	want := 15*instance.Billing.ComputePricePerSecond + 35*instance.Billing.DiskPricePerSecond
	if instance.Billing.CurrentCost != want {
		t.Fatalf("cost = %v, want %v", instance.Billing.CurrentCost, want)
	}
}

func TestBillingDistinguishesMissingPricesFromFreeCompute(t *testing.T) {
	missing := billingForTarget(domain.CloudCapacityTarget{Configuration: map[string]any{"diskSizeGiB": 30}}, time.Now())
	if missing.PricingStatus != "incomplete" || len(missing.MissingPrices) != 2 {
		t.Fatalf("missing tariffs presented as calibrated: %+v", missing)
	}
	priced := billingForTarget(domain.CloudCapacityTarget{Configuration: map[string]any{
		"pricePerHour": 0.032, "diskSizeGiB": 30, "diskPricePerGiBMonth": 0.04,
		"priceCurrency": "USD", "priceSource": "gcp-public-catalog",
	}}, time.Now())
	if priced.PricingStatus != "estimated" || len(priced.MissingPrices) != 0 || priced.PriceCurrency != "USD" || priced.PriceSource != "gcp-public-catalog" {
		t.Fatalf("tariff provenance lost: %+v", priced)
	}
}

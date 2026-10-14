package gcp

import (
	"encoding/json"
	"testing"
)

func TestZonalBootDiskPricesRejectOtherStorageProducts(t *testing.T) {
	for _, pair := range [][2]string{
		{"pd-standard", "extreme pd capacity"},
		{"pd-standard", "regional standard pd capacity"},
		{"pd-balanced", "regional balanced pd capacity"},
		{"pd-balanced", "hyperdisk balanced capacity"},
	} {
		if diskSKUDescription(pair[0], pair[1]) {
			t.Fatalf("matched other disk product: %v", pair)
		}
	}
	for _, pair := range [][2]string{{"pd-standard", "storage pd capacity"}, {"pd-balanced", "balanced pd capacity"}, {"pd-ssd", "ssd backed pd capacity"}} {
		if !diskSKUDescription(pair[0], pair[1]) {
			t.Fatalf("rejected zonal disk: %v", pair)
		}
	}
}

func TestDiskFreeAllowanceDoesNotHidePaidMarginalRate(t *testing.T) {
	var sku billingSKU
	if err := json.Unmarshal(
		[]byte(
			("{\"pricingInfo\":[{\"pricingExpression\":{\"usageUnit\":\"GiBy.mo\",\"tieredRates\":[{\"startUsageAmount\":0,\"un" +
				"itPrice\":{\"currencyCode\":\"USD\",\"units\":\"0\",\"nanos\":0}},{\"startUsageAmount\":30,\"unitPrice\":{\"currency" +
				"Code\":\"USD\",\"units\":\"0\",\"nanos\":40000000}}]}}]}"),
		),
		&sku,
	); err != nil {
		t.Fatal(err)
	}
	price, currency := skuUnitPrice(sku)
	if price != 0.04 || currency != "USD" {
		t.Fatalf("free allowance treated as unlimited: %v %s", price, currency)
	}
}

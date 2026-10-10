package gcp

import (
	"math"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestE2SharedCorePricesUseFractionalAllocationNotGuestCPUCount(t *testing.T) {
	catalog := domain.CloudCatalog{Region: "us-central1", Machines: []domain.CloudMachineOffering{
		{ProviderTypeID: "e2-micro", Family: "e2", VCPU: 2, MemoryMiB: 1024},
		{ProviderTypeID: "e2-small", Family: "e2", VCPU: 2, MemoryMiB: 2048},
		{ProviderTypeID: "e2-medium", Family: "e2", VCPU: 2, MemoryMiB: 4096},
		{ProviderTypeID: "e2-standard-2", Family: "e2", VCPU: 2, MemoryMiB: 8192},
	}}
	applyPrices([]billingSKU{priceSKU("E2 Instance Core running in Americas", "CPU", 0.02), priceSKU("E2 Instance Ram running in Americas", "RAM", 0.003)}, &catalog)
	for i, want := range []float64{0.008, 0.016, 0.032, 0.064} {
		if got := catalog.Machines[i].PricePerHour; math.Abs(got-want) > 1e-12 {
			t.Fatalf("%s: price=%f want=%f", catalog.Machines[i].ProviderTypeID, got, want)
		}
	}
}

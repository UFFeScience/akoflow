package gcp

import "testing"

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

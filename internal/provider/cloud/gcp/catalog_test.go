package gcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDiscoverNormalizesLiveCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"access_token":"token"}`))
		case strings.HasSuffix(r.URL.Path, "/aggregated/machineTypes"):
			_, _ = w.Write([]byte(`{
				"items": {
					"zones/us-central1-a": {"machineTypes": [{
						"name": "c3-standard-8", "guestCpus": 8, "memoryMb": 32768,
						"zone": "zones/us-central1-a", "architecture": "X86_64"
					}]},
					"zones/us-east1-b": {"machineTypes": [{
						"name": "e2-standard-4", "guestCpus": 4, "memoryMb": 16384,
						"zone": "zones/us-east1-b"
					}]}
				}
			}`))
		case strings.HasSuffix(r.URL.Path, "/aggregated/diskTypes"):
			_, _ = w.Write([]byte(`{"items":{"zones/us-central1-a":{"diskTypes":[{"name":"pd-balanced","zone":"zones/us-central1-a"}]}}}`))
		case r.URL.Path == "/skus":
			_, _ = w.Write([]byte(`{"skus":[]}`))
		case strings.Contains(r.URL.Path, "/global/images"):
			_, _ = w.Write([]byte(`{"items":[{"name":"ubuntu-2404-v1","selfLink":"projects/ubuntu-os-cloud/global/images/ubuntu-2404-v1","architecture":"X86_64","status":"READY","diskSizeGb":"10"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	credential := testCredential(t, server.URL+"/token")
	catalog := New(server.Client())
	catalog.computeEndpoint = server.URL
	catalog.billingEndpoint = server.URL + "/skus"
	result, err := catalog.Discover(context.Background(), domain.EnvironmentConnection{Configuration: map[string]any{"provider": "gcp", "projectId": "science", "region": "us-central1"}}, credential)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "live" || len(result.Machines) != 1 || result.Machines[0].ProviderTypeID != "c3-standard-8" {
		t.Fatalf("unexpected machines %#v", result.Machines)
	}
	if len(result.Disks) != 1 || len(result.Images) != 4 {
		t.Fatalf("unexpected catalog disks=%d images=%d", len(result.Disks), len(result.Images))
	}
}

func TestValidateCredentialChecksProjectWithoutDiscoveringCatalog(t *testing.T) {
	var projectCalls, catalogCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"access_token":"token"}`))
		case r.URL.Path == "/projects/science":
			projectCalls++
			_, _ = w.Write([]byte(`{}`))
		default:
			catalogCalls++
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	catalog := New(server.Client())
	catalog.computeEndpoint = server.URL
	err := catalog.ValidateCredential(context.Background(), domain.EnvironmentConnection{
		Configuration: map[string]any{"projectId": "science"},
	}, testCredential(t, server.URL+"/token"))
	if err != nil || projectCalls != 1 || catalogCalls != 0 {
		t.Fatalf("access check err=%v projectCalls=%d catalogCalls=%d", err, projectCalls, catalogCalls)
	}
}

func TestApplyPricesCombinesMachineAndDiskSKUs(t *testing.T) {
	result := domain.CloudCatalog{
		Region:   "us-central1",
		Machines: []domain.CloudMachineOffering{{Family: "e2", VCPU: 2, MemoryMiB: 4096}},
		Disks:    []domain.CloudDiskOffering{{ProviderTypeID: "pd-balanced"}},
	}
	core := priceSKU("E2 Instance Core running in Americas", "CPU", 0.02)
	ram := priceSKU("E2 Instance Ram running in Americas", "RAM", 0.003)
	disk := priceSKU("Balanced PD Capacity", "SSD", 0.1)
	applyPrices([]billingSKU{core, ram, disk}, &result)

	if got, want := result.Machines[0].PricePerHour, 0.052; math.Abs(got-want) > 1e-12 {
		t.Fatalf("machine price = %f, want %f", got, want)
	}
	if got, want := result.Machines[0].PricePerMinute, 0.052/60; math.Abs(got-want) > 1e-12 {
		t.Fatalf("machine minute price = %f, want %f", got, want)
	}
	if got, want := result.Disks[0].PricePerGiBMonth, 0.1; math.Abs(got-want) > 1e-12 {
		t.Fatalf("disk price = %f, want %f", got, want)
	}
}

func priceSKU(description, resourceGroup string, price float64) billingSKU {
	value := billingSKU{Description: description, ServiceRegions: []string{"us-central1"}}
	value.Category.ResourceGroup = resourceGroup
	value.Category.UsageType = "OnDemand"
	value.PricingInfo = make([]struct {
		PricingExpression struct {
			UsageUnit   string `json:"usageUnit"`
			TieredRates []struct {
				StartUsageAmount float64 `json:"startUsageAmount"`
				UnitPrice        struct {
					CurrencyCode string `json:"currencyCode"`
					Units        string `json:"units"`
					Nanos        int64  `json:"nanos"`
				} `json:"unitPrice"`
			} `json:"tieredRates"`
		} `json:"pricingExpression"`
	}, 1)
	value.PricingInfo[0].PricingExpression.TieredRates = make([]struct {
		StartUsageAmount float64 `json:"startUsageAmount"`
		UnitPrice        struct {
			CurrencyCode string `json:"currencyCode"`
			Units        string `json:"units"`
			Nanos        int64  `json:"nanos"`
		} `json:"unitPrice"`
	}, 1)
	value.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice.CurrencyCode = "USD"
	value.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice.Nanos = int64(price * 1e9)
	return value
}

func TestMachineCatalogFollowsProviderPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "second page" {
			_, _ = w.Write([]byte(`{"items":{"zones/us-central1-b":{"machineTypes":[{"name":"c3-standard-8","guestCpus":8,"memoryMb":32768,"zone":"zones/us-central1-b"}]}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"nextPageToken":"second page","items":{"zones/us-central1-a":{"machineTypes":[{"name":"c3-standard-8","guestCpus":8,"memoryMb":32768,"zone":"zones/us-central1-a"}]}}}`))
	}))
	defer server.Close()
	catalog := New(server.Client())
	catalog.computeEndpoint = server.URL
	result := domain.CloudCatalog{Region: "us-central1"}
	if err := catalog.machines(context.Background(), "token", "science", &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Machines) != 1 || len(result.Machines[0].Zones) != 2 {
		t.Fatalf("machines = %#v", result.Machines)
	}
}

func testCredential(t *testing.T, tokenURI string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	value, _ := json.Marshal(map[string]string{"project_id": "science", "client_email": "worker@example.test", "private_key": string(encoded), "token_uri": tokenURI})
	return value
}

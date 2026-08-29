package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDiscoveryReadsNodesAndKindLoginTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("path=%q authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"items":[
			{"metadata":{"name":"kind-control-plane","labels":{"role":"control"}},"status":{"nodeInfo":{"architecture":"amd64","containerRuntimeVersion":"containerd://1"},"allocatable":{"cpu":"3500m","memory":"2Gi"},"conditions":[{"Type":"Ready","Status":"True"}]}},
			{"metadata":{"name":"kind-worker"},"status":{"nodeInfo":{"architecture":"arm64","containerRuntimeVersion":"containerd://1"},"allocatable":{"cpu":"2","memory":"512Mi"},"conditions":[{"Type":"Ready","Status":"False"}]}}
		]}`))
	}))
	defer server.Close()
	discovery := NewDiscovery()
	result, err := discovery.DiscoverConnection(context.Background(), domain.EnvironmentConnection{
		Endpoint: server.URL, Configuration: map[string]any{"bearerToken": "token", "context": "kind-science"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || result.Metadata["nodeCount"] != 2 || result.Metadata["readyNodeCount"] != 1 || result.LoginNode == nil {
		t.Fatalf("result=%+v", result)
	}
	if result.LoginNode.Name != "kind-control-plane" || result.LoginNode.CPUCores != 4 || result.LoginNode.MemoryBytes != 2*1024*1024*1024 || result.LoginNode.Metadata["interactiveDockerContainer"] != "science-control-plane" {
		t.Fatalf("login=%+v", result.LoginNode)
	}
}

func TestDiscoveryReportsConfigurationAPIAndDecodeErrors(t *testing.T) {
	discovery := NewDiscovery()
	if _, err := discovery.DiscoverConnection(context.Background(), domain.EnvironmentConnection{}); err == nil {
		t.Fatal("missing endpoint and credential must fail")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if _, err := discovery.DiscoverConnection(context.Background(), domain.EnvironmentConnection{Endpoint: server.URL, Configuration: map[string]any{"bearerToken": "token"}}); err == nil {
		t.Fatal("API failure expected")
	}
	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }))
	defer badJSON.Close()
	if _, err := discovery.DiscoverConnection(context.Background(), domain.EnvironmentConnection{Endpoint: badJSON.URL, Configuration: map[string]any{"bearerToken": "token"}}); err == nil || !strings.Contains(err.Error(), "decode Kubernetes nodes") {
		t.Fatalf("decode error=%v", err)
	}
}

func TestKubernetesResourceConversions(t *testing.T) {
	if kubernetesCPUCores("1500m") != 2 || kubernetesCPUCores("2") != 2 || kubernetesCPUCores("bad") != 0 {
		t.Fatal("CPU conversion failed")
	}
	if kubernetesBytes("1Ki") != 1024 || kubernetesBytes("2Mi") != 2*1024*1024 || kubernetesBytes("1.5Gi") != int64(1.5*1024*1024*1024) || kubernetesBytes("42") != 42 {
		t.Fatal("memory conversion failed")
	}
	keys := mapKeys(map[string]bool{"amd64": true, "": true})
	if len(keys) != 1 || keys[0] != "amd64" {
		t.Fatalf("keys=%+v", keys)
	}
}

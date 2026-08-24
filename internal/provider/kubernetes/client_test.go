package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientUsesBearerTokenAndRESTPaths(t *testing.T) {
	requests := make([]string, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
		}
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":{"active":1}}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Endpoint: server.URL, Token: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Create(context.Background(), "science", "jobs", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(context.Background(), "science", "jobs", "job-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.List(context.Background(), "science", "pods", "job-name=job-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Logs(context.Background(), "science", "pod-1", "observer"); err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(context.Background(), "science", "services", "svc-1"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"POST /apis/batch/v1/namespaces/science/jobs",
		"GET /apis/batch/v1/namespaces/science/jobs/job-1",
		"GET /api/v1/namespaces/science/pods?labelSelector=job-name%3Djob-1",
		"GET /api/v1/namespaces/science/pods/pod-1/log?container=observer",
		"DELETE /api/v1/namespaces/science/services/svc-1",
	}
	for index := range want {
		if requests[index] != want[index] {
			t.Fatalf("request[%d]=%q want %q", index, requests[index], want[index])
		}
	}
}

func TestClientRequiresEndpointAndToken(t *testing.T) {
	if _, err := NewClient(ClientConfig{}); err == nil {
		t.Fatal("expected missing endpoint error")
	}
	if _, err := NewClient(ClientConfig{Endpoint: "https://kubernetes.example"}); err == nil {
		t.Fatal("expected missing token error")
	}
}

func TestClientAcceptsHistoricalHostWithoutScheme(t *testing.T) {
	client, err := NewClient(ClientConfig{
		Endpoint: "kind-control-plane:6443", Token: "secret", HTTPClient: http.DefaultClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "https://kind-control-plane:6443" {
		t.Fatalf("endpoint=%q", client.endpoint)
	}
}

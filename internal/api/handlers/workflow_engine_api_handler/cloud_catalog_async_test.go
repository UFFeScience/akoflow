package workflow_engine_api_handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type asyncCloudCatalogStub struct {
	started chan struct{}
	release chan struct{}
}

func (s *asyncCloudCatalogStub) Discover(ctx context.Context, _ string) (domain.CloudCatalog, error) {
	close(s.started)
	select {
	case <-s.release:
	case <-ctx.Done():
		return domain.CloudCatalog{}, ctx.Err()
	}
	return domain.CloudCatalog{}, nil
}
func (*asyncCloudCatalogStub) Cached(context.Context, string) (*domain.CloudCatalog, error) {
	return nil, nil
}
func (*asyncCloudCatalogStub) CheckAccess(context.Context, domain.EnvironmentConnection, []byte) error {
	return nil
}
func (*asyncCloudCatalogStub) Validate(context.Context, domain.EnvironmentConnection, []byte) (domain.CloudCatalog, error) {
	return domain.CloudCatalog{}, nil
}

func TestRefreshCloudCatalogAsyncReturnsBeforeDiscoveryCompletes(t *testing.T) {
	stub := &asyncCloudCatalogStub{started: make(chan struct{}), release: make(chan struct{})}
	handler := &Handler{cloudCatalog: stub}
	request := httptest.NewRequest(http.MethodPost, "/akoflow-api/environments/cloud/cloud-catalog/refresh/?async=true", nil)
	request.SetPathValue("environmentId", "cloud")
	response := httptest.NewRecorder()
	handler.RefreshCloudCatalog(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	select {
	case <-stub.started:
	case <-time.After(time.Second):
		t.Fatal("discovery did not start")
	}
	close(stub.release)
}

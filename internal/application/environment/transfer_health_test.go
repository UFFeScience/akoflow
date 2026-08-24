package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestConnectorHealthIsExplicitAndReportsTestedOperation(t *testing.T) {
	called := false
	service := NewConnectorBindingChecker(map[domain.TransferConnector]BindingProbe{
		domain.TransferConnectorS3: func(_ context.Context, binding domain.ConnectorBinding) (string, error) {
			called = binding.CredentialRef == "lab-s3"
			return "head-object", nil
		},
	})
	health, err := service.CheckConnectorBinding(context.Background(), domain.ConnectorBinding{Connector: domain.TransferConnectorS3, CredentialRef: "lab-s3"})
	if err != nil || !called || !health.Healthy || health.Operation != "head-object" || health.CheckedAt.IsZero() {
		t.Fatalf("health=%+v err=%v called=%v", health, err, called)
	}
}

func TestConnectorHealthPreservesUnavailableReason(t *testing.T) {
	service := NewConnectorBindingChecker(map[domain.TransferConnector]BindingProbe{
		domain.TransferConnectorHTTP: func(context.Context, domain.ConnectorBinding) (string, error) { return "head", errors.New("forbidden") },
	})
	health, err := service.CheckConnectorBinding(context.Background(), domain.ConnectorBinding{Connector: domain.TransferConnectorHTTP})
	if err == nil || health.Healthy || health.Operation != "head" || health.Reason != "forbidden" {
		t.Fatalf("health=%+v err=%v", health, err)
	}
}

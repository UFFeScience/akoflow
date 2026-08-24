package main

import (
	"context"
	"testing"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
)

type instanceStoreStub struct {
	value *domaininstance.Instance
	err   error
}

func (store *instanceStoreStub) Find(context.Context) (*domaininstance.Instance, error) {
	return store.value, store.err
}

func (store *instanceStoreStub) Save(_ context.Context, value domaininstance.Instance) error {
	store.value = &value
	return store.err
}

func (store *instanceStoreStub) FindPreferences(context.Context, string) (*domaininstance.UserPreferences, error) {
	return nil, store.err
}

func (store *instanceStoreStub) SavePreferences(context.Context, domaininstance.UserPreferences) error {
	return store.err
}

func TestEnsureSystemInstanceKeepsExistingIdentity(t *testing.T) {
	existing := &domaininstance.Instance{ID: "existing", Name: "Existing"}
	store := &instanceStoreStub{value: existing}
	if err := ensureSystemInstance(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if store.value != existing {
		t.Fatal("existing instance must not be replaced")
	}
}

func TestEnsureSystemInstanceCreatesHostIdentity(t *testing.T) {
	store := &instanceStoreStub{}
	if err := ensureSystemInstance(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if store.value == nil || store.value.ID == "" || store.value.Name == "" {
		t.Fatalf("invalid detected instance: %+v", store.value)
	}
	if store.value.Organization != "" || store.value.Location != "" {
		t.Fatalf("host identity must not infer organization or location: %+v", store.value)
	}
}

package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
)

var invalidInstanceID = regexp.MustCompile(`[^a-z0-9]+`)

func ensureSystemInstance(ctx context.Context, store ports.InstanceStore) error {
	current, err := store.Find(ctx)
	if err != nil {
		return fmt.Errorf("find system instance: %w", err)
	}
	if current != nil {
		return nil
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("detect system hostname: %w", err)
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "akoflow"
	}
	identifier := strings.Trim(invalidInstanceID.ReplaceAllString(strings.ToLower(hostname), "-"), "-")
	if identifier == "" {
		identifier = "akoflow"
	}
	if err := store.Save(ctx, domaininstance.Instance{
		ID:          identifier,
		Name:        hostname,
		Description: "AkôFlow control plane detected on " + hostname,
	}); err != nil {
		return fmt.Errorf("initialize system instance: %w", err)
	}
	return nil
}

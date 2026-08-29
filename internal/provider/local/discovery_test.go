package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type executorStub struct {
	output []byte
	err    error
	calls  int
}

func (s *executorStub) Run(context.Context, string, []string, []byte) ([]byte, error) {
	s.calls++
	return s.output, s.err
}

func TestDiscoveryParsesBoundedMachineMetadata(t *testing.T) {
	executor := &executorStub{output: []byte("arch=x86_64\ncpu=8\nmemKiB=1024\ndiskTotalKiB=2048\ndiskAvailableKiB=512\nruntime=docker\nruntime=apptainer\ninvalid\n")}
	discovery := NewDiscovery(executor)
	result, err := discovery.DiscoverConnection(context.Background(), domain.EnvironmentConnection{})
	if err != nil || !result.Available {
		t.Fatalf("DiscoverConnection()=%#v,%v", result, err)
	}
	if result.Metadata["architecture"] != "x86_64" || result.Metadata["cpuCores"] != 8 || result.Metadata["memoryBytes"] != int64(1024*1024) || result.Metadata["diskTotalBytes"] != int64(2048*1024) {
		t.Fatalf("metadata=%#v", result.Metadata)
	}
	runtimes := result.Metadata["containerRuntimes"].([]string)
	if len(runtimes) != 2 || runtimes[1] != "apptainer" {
		t.Fatalf("runtimes=%#v", runtimes)
	}
	if kibibytes("bad") != 0 || kibibytes("2") != 2048 {
		t.Fatal("kibibytes mismatch")
	}
}

func TestDiscoveryAndConnectionProbeReportExecutorFailure(t *testing.T) {
	executor := &executorStub{err: fmt.Errorf("unavailable")}
	if _, err := NewDiscovery(executor).DiscoverConnection(context.Background(), domain.EnvironmentConnection{}); err == nil {
		t.Fatal("expected discovery error")
	}
	health := NewConnectionProber(executor).Probe(context.Background(), domain.EnvironmentConnection{})
	if health.Healthy || health.Message == "" {
		t.Fatalf("health=%#v", health)
	}
	executor.err = nil
	health = NewConnectionProber(executor).Probe(context.Background(), domain.EnvironmentConnection{})
	if !health.Healthy {
		t.Fatalf("health=%#v", health)
	}
	if NewDiscovery().executor == nil || NewConnectionProber().executor == nil {
		t.Fatal("default executors missing")
	}
}

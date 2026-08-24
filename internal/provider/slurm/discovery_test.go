package slurm

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestParseDiscoveryBuildsUniqueNodesWithPartitionMembership(t *testing.T) {
	result := parseDiscovery([]byte(`FACT|architecture|x86_64
FACT|hostname|login.plafrim.fr
FACT|loginCpuCores|16
FACT|loginMemoryKiB|32768
PARTITION|routage*|up|2|48|192000|3-00:00:00
PARTITION|preempt|up|1|48|192000|3-00:00:00
NODE|routage*|bora001|idle|48|192000|1000|1|avx2|gpu:a100:2|none
NODE|preempt|bora001|idle|48|192000|1000|1|avx2|gpu:a100:2|none
NODE|routage*|bora002|alloc|48|192000|1000|1|(null)|(null)|job running
FILESYSTEM|/dev/home|lustre|1000|200|800|/home
`))
	partitions := result.Metadata["partitions"].([]map[string]any)
	nodes := result.Metadata["nodes"].([]map[string]any)
	if !result.Available || len(partitions) != 2 || len(nodes) != 2 {
		t.Fatalf("result=%+v", result)
	}
	if partitions[0]["name"] != "routage" || partitions[0]["default"] != true {
		t.Fatalf("partition=%+v", partitions[0])
	}
	membership := nodes[0]["partitions"].([]string)
	if len(membership) != 2 || membership[0] != "routage" || membership[1] != "preempt" {
		t.Fatalf("node=%+v", nodes[0])
	}
	if nodes[0]["cpuCores"] != int64(48) || nodes[0]["memoryMiB"] != int64(192000) {
		t.Fatalf("node capacities=%+v", nodes[0])
	}
	if result.LoginNode == nil || result.LoginNode.Name != "login.plafrim.fr" || result.LoginNode.CPUCores != 16 || result.LoginNode.MemoryBytes != 32768*1024 {
		t.Fatalf("login=%+v", result.LoginNode)
	}
}

func TestParseDiscoveryWarnsAboutMalformedRecords(t *testing.T) {
	result := parseDiscovery([]byte("PARTITION|broken\nNODE|broken\n"))
	if result.Available || len(result.Warnings) != 2 {
		t.Fatalf("result=%+v", result)
	}
}

func TestParseDiscoveryIncludesTransferCapabilitiesAndPaths(t *testing.T) {
	result := parseDiscovery([]byte(`PARTITION|short*|up|1|4|16000|1:00:00
TRANSFER_TOOL|rsync|available
TRANSFER_TOOL|scp|available
TRANSFER_TOOL|sftp|unavailable
TRANSFER_TOOL|curl|available
TRANSFER_TOOL|apptainer|available
TRANSFER_TOOL|sha256sum|available
TRANSFER_HTTPS|unavailable
TRANSFER_PATH|home|/home/test|true|1000
`))
	if !result.Transfer.Connectors[domain.TransferConnectorRsync].Available || result.Transfer.Connectors[domain.TransferConnectorSFTP].Available {
		t.Fatalf("unexpected connector capabilities: %+v", result.Transfer.Connectors)
	}
	if result.Transfer.OutboundHTTPS.Available || result.Transfer.OutboundHTTPS.Reason == "" {
		t.Fatalf("expected unavailable HTTPS reason: %+v", result.Transfer.OutboundHTTPS)
	}
	if !result.Transfer.ContainerRuntime.Available || !result.Transfer.Checksum.Available || len(result.Transfer.Paths) != 1 {
		t.Fatalf("unexpected transfer observation: %+v", result.Transfer)
	}
	path := result.Transfer.Paths[0]
	if !path.Writable || !path.LoginNodeVisible || path.AvailableBytes != 1000 || path.ComputeNodeVisible != nil {
		t.Fatalf("unexpected transfer path: %+v", path)
	}
}

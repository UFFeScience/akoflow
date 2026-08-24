package slurm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

// Discovery collects a bounded inventory without connecting to compute nodes.
type Discovery struct{ executor runtimecommon.CommandExecutor }

func NewDiscovery(executors ...runtimecommon.CommandExecutor) *Discovery {
	executor := runtimecommon.CommandExecutor(runtimecommon.OSCommandExecutor{})
	if len(executors) > 0 && executors[0] != nil {
		executor = executors[0]
	}
	return &Discovery{executor: executor}
}

func (d *Discovery) DiscoverConnection(ctx context.Context, connection domain.EnvironmentConnection) (ports.ConnectionDiscovery, error) {
	if connection.Type != domain.ConnectionSSH && connection.Type != domain.ConnectionAgent && connection.Type != domain.ConnectionLocal {
		return ports.ConnectionDiscovery{}, fmt.Errorf("unsupported SLURM connection type %q", connection.Type)
	}
	executor := d.executor
	if connection.Type == domain.ConnectionSSH {
		executor = runtimecommon.SSHCommandExecutor{Executor: d.executor, Endpoint: connection.Endpoint,
			Username: connection.Username, Port: configInt(connection.Configuration, "port"),
			IdentityFile: credentialFile(connection.CredentialRef), ProxyCommand: configString(connection.Configuration, "proxyCommand"),
			HostKeyAlias:   configString(connection.Configuration, "hostKeyAlias"),
			KnownHostsFile: knownHostsFile(connection),
			ForwardAgent:   configBool(connection.Configuration, "forwardAgent", false)}
	}
	script := strings.Join([]string{
		`printf 'FACT|architecture|'; uname -m`,
		`printf 'FACT|hostname|'; hostname -f 2>/dev/null || hostname`,
		`printf 'FACT|kernel|'; uname -sr`,
		`printf 'FACT|os|'; (awk -F= '$1=="PRETTY_NAME" {gsub(/^"|"$/, "", $2); print $2}' /etc/os-release 2>/dev/null || true)`,
		`printf 'FACT|workingDirectory|'; pwd -P`,
		`printf 'FACT|homeDirectory|'; printf '%s\n' "$HOME"`,
		`printf 'FACT|temporaryDirectory|'; printf '%s\n' "${TMPDIR:-/tmp}"`,
		`printf 'FACT|loginCpuCores|'; (getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || printf '0\n')`,
		`printf 'FACT|loginMemoryKiB|'; awk '/^MemTotal:/ {print $2}' /proc/meminfo 2>/dev/null`,
		`printf 'FACT|slurmVersion|'; (sinfo --version 2>/dev/null || true)`,
		`printf 'FACT|queueLength|'; squeue -h 2>/dev/null | wc -l | tr -d ' '`,
		`(apptainer --version || singularity --version) 2>/dev/null | sed 's/^/FACT|containerRuntime|/'`,
		`for tool in rsync scp sftp curl apptainer singularity sha256sum; do if command -v "$tool" >/dev/null 2>&1; then printf 'TRANSFER_TOOL|%s|available\n' "$tool"; else printf 'TRANSFER_TOOL|%s|unavailable\n' "$tool"; fi; done`,
		`if command -v curl >/dev/null 2>&1 && curl --head --silent --show-error --max-time 5 https://example.com >/dev/null 2>&1; then printf 'TRANSFER_HTTPS|available\n'; else printf 'TRANSFER_HTTPS|unavailable\n'; fi`,
		transferPathScript(),
		`sinfo -h -o 'PARTITION|%P|%a|%D|%c|%m|%l'`,
		`sinfo -h -N -o 'NODE|%P|%N|%T|%c|%m|%d|%w|%f|%G|%E'`,
		`df -PkT "$HOME" "${TMPDIR:-/tmp}" "$(pwd -P)" 2>/dev/null | awk 'NR>1 {printf "FILESYSTEM|%s|%s|%s|%s|%s|%s\n", $1, $2, $3, $4, $5, $7}' | sort -u`,
	}, "; ")
	// Send the generated script as base64. The discovery probe deliberately
	// contains awk programs, substitutions and parentheses; passing it through
	// both the local SSH command line and the remote login shell otherwise
	// makes those expressions susceptible to a second round of shell parsing.
	encodedScript := base64.StdEncoding.EncodeToString([]byte(script))
	output, err := executor.Run(ctx, "/bin/sh", []string{"-c", "printf %s " + encodedScript + " | base64 -d | /bin/sh"}, nil)
	if err != nil {
		return ports.ConnectionDiscovery{}, fmt.Errorf("SLURM discovery: %w", err)
	}
	result := parseDiscovery(output)
	addConfiguredTransferPaths(&result.Transfer, connection.Configuration)
	result.Metadata["transferCapabilities"] = result.Transfer
	return result, nil
}

func parseDiscovery(output []byte) ports.ConnectionDiscovery {
	metadata := map[string]any{}
	partitions := []map[string]any{}
	nodesByName := map[string]map[string]any{}
	nodeOrder := []string{}
	filesystems := []map[string]any{}
	warnings := []string{}
	transfer := transferCapabilities(time.Now().UTC())
	for _, raw := range strings.Split(string(output), "\n") {
		fields := strings.Split(strings.TrimSpace(raw), "|")
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		switch fields[0] {
		case "FACT":
			if len(fields) >= 3 {
				metadata[fields[1]] = strings.Join(fields[2:], "|")
			}
		case "PARTITION":
			if len(fields) < 7 {
				warnings = append(warnings, "ignored malformed partition record")
				continue
			}
			partitions = append(partitions, map[string]any{"name": cleanPartition(fields[1]),
				"default": strings.HasSuffix(fields[1], "*"), "availability": fields[2], "nodeCount": integer(fields[3]),
				"cpuCoresPerNode": integer(fields[4]), "memoryMiBPerNode": integer(fields[5]), "timeLimit": fields[6]})
		case "NODE":
			if len(fields) < 11 {
				warnings = append(warnings, "ignored malformed node record")
				continue
			}
			name, partition := fields[2], cleanPartition(fields[1])
			node := nodesByName[name]
			if node == nil {
				node = map[string]any{"name": name, "state": fields[3], "cpuCores": integer(fields[4]),
					"memoryMiB": integer(fields[5]), "temporaryDiskMiB": integer(fields[6]), "weight": integer(fields[7]),
					"features": csv(fields[8]), "gres": csv(fields[9]), "reason": emptyIfNone(fields[10]), "partitions": []string{}}
				nodesByName[name], nodeOrder = node, append(nodeOrder, name)
			}
			node["partitions"] = appendUnique(node["partitions"].([]string), partition)
		case "FILESYSTEM":
			if len(fields) >= 7 {
				filesystems = append(filesystems, map[string]any{"device": fields[1], "type": fields[2],
					"capacityKiB": integer(fields[3]), "usedKiB": integer(fields[4]), "availableKiB": integer(fields[5]), "mountPoint": fields[6]})
			}
		default:
			handleTransferRecord(&transfer, fields)
		}
	}
	nodes := orderedNodes(nodesByName, nodeOrder)
	metadata["partitions"], metadata["nodes"], metadata["filesystems"] = partitions, nodes, filesystems
	discovered := discoveredNodes(nodes, stringValue(metadata["architecture"]))
	login := discoveredLoginNode(metadata, filesystems)
	metadata["transferCapabilities"] = transfer
	return ports.ConnectionDiscovery{Available: len(partitions) > 0, Metadata: metadata, Warnings: warnings, Nodes: discovered, LoginNode: login, Transfer: transfer}
}

func transferPathScript() string {
	return `for pair in "home:$HOME" "temporary:${TMPDIR:-/tmp}" "working:$(pwd -P)"; do ` +
		`kind=${pair%%:*}; path=${pair#*:}; available=$(df -Pk "$path" 2>/dev/null | ` +
		`awk 'NR==2 {print $4 * 1024}'); if test -w "$path"; then writable=true; ` +
		`else writable=false; fi; printf 'TRANSFER_PATH|%s|%s|%s|%s\n' ` +
		`"$kind" "$path" "$writable" "${available:-0}"; done`
}

func handleTransferRecord(transfer *domain.TransferCapabilities, fields []string) {
	switch fields[0] {
	case "TRANSFER_TOOL":
		if len(fields) >= 3 {
			setTransferTool(transfer, fields[1], fields[2] == "available")
		}
	case "TRANSFER_HTTPS":
		if len(fields) >= 2 {
			transfer.OutboundHTTPS.Available = fields[1] == "available"
			if !transfer.OutboundHTTPS.Available {
				transfer.OutboundHTTPS.Reason = "outbound HTTPS probe failed"
			}
		}
	case "TRANSFER_PATH":
		if len(fields) >= 5 {
			transfer.Paths = append(transfer.Paths, transferPath(fields))
		}
	}
}

func transferPath(fields []string) domain.TransferPath {
	return domain.TransferPath{Path: fields[2], Kind: fields[1], Writable: fields[3] == "true", LoginNodeVisible: true, AvailableBytes: integer(fields[4])}
}

func orderedNodes(byName map[string]map[string]any, order []string) []map[string]any {
	nodes := make([]map[string]any, 0, len(order))
	for _, name := range order {
		nodes = append(nodes, byName[name])
	}
	return nodes
}

func discoveredNodes(nodes []map[string]any, architecture string) []ports.DiscoveredNode {
	discovered := make([]ports.DiscoveredNode, 0, len(nodes))
	for _, node := range nodes {
		discovered = append(discovered, discoveredNode(node, architecture))
	}
	return discovered
}

func discoveredNode(node map[string]any, architecture string) ports.DiscoveredNode {
	return ports.DiscoveredNode{
		Name:             node["name"].(string),
		State:            node["state"].(string),
		CPUCores:         int(node["cpuCores"].(int64)),
		MemoryBytes:      node["memoryMiB"].(int64) * 1024 * 1024,
		StorageBytes:     node["temporaryDiskMiB"].(int64) * 1024 * 1024,
		Architecture:     architecture,
		Partitions:       node["partitions"].([]string),
		Features:         node["features"].([]string),
		GenericResources: node["gres"].([]string),
		Reason:           node["reason"].(string),
		Metadata:         node,
	}
}

func transferCapabilities(now time.Time) domain.TransferCapabilities {
	fresh := now.Add(15 * time.Minute)
	makeObservation := func(reason string) domain.CapabilityObservation {
		return domain.CapabilityObservation{Reason: reason, ObservedAt: now, FreshUntil: fresh}
	}
	return domain.TransferCapabilities{ObservedAt: now, FreshUntil: fresh,
		Connectors: map[domain.TransferConnector]domain.CapabilityObservation{
			domain.TransferConnectorRsync: makeObservation("rsync is not installed"),
			domain.TransferConnectorSCP:   makeObservation("scp is not installed"),
			domain.TransferConnectorSFTP:  makeObservation("sftp is not installed"),
			domain.TransferConnectorHTTP:  makeObservation("curl is not installed"),
		}, OutboundHTTPS: makeObservation("outbound HTTPS was not probed"),
		ContainerRuntime: makeObservation("apptainer or singularity is not installed"),
		Checksum:         makeObservation("sha256sum is not installed")}
}

func setTransferTool(capabilities *domain.TransferCapabilities, tool string, available bool) {
	set := func(connector domain.TransferConnector, reason string) {
		capabilities.Connectors[connector] = domain.CapabilityObservation{Available: available, Reason: reason, ObservedAt: capabilities.ObservedAt, FreshUntil: capabilities.FreshUntil}
	}
	switch tool {
	case "rsync":
		set(domain.TransferConnectorRsync, unavailableReason(available, "rsync is not installed"))
	case "scp":
		set(domain.TransferConnectorSCP, unavailableReason(available, "scp is not installed"))
	case "sftp":
		set(domain.TransferConnectorSFTP, unavailableReason(available, "sftp is not installed"))
	case "curl":
		set(domain.TransferConnectorHTTP, unavailableReason(available, "curl is not installed"))
	case "apptainer", "singularity":
		capabilities.ContainerRuntime = capabilityObservation(capabilities, available, "apptainer or singularity is not installed")
	case "sha256sum":
		capabilities.Checksum = capabilityObservation(capabilities, available, "sha256sum is not installed")
	}
}

func capabilityObservation(capabilities *domain.TransferCapabilities, available bool, unavailable string) domain.CapabilityObservation {
	return domain.CapabilityObservation{Available: available, Reason: unavailableReason(available, unavailable), ObservedAt: capabilities.ObservedAt, FreshUntil: capabilities.FreshUntil}
}

func unavailableReason(available bool, reason string) string {
	if available {
		return ""
	}
	return reason
}

func addConfiguredTransferPaths(capabilities *domain.TransferCapabilities, configuration map[string]any) {
	values, _ := configuration["transferPaths"].([]any)
	for _, raw := range values {
		if path, ok := raw.(string); ok && strings.TrimSpace(path) != "" {
			capabilities.Paths = append(capabilities.Paths, domain.TransferPath{Path: path, Kind: "configured", Reason: "requires explicit path health check"})
		}
	}
}

func discoveredLoginNode(metadata map[string]any, filesystems []map[string]any) *ports.DiscoveredLoginNode {
	name, _ := metadata["hostname"].(string)
	if strings.TrimSpace(name) == "" {
		return nil
	}
	architecture, _ := metadata["architecture"].(string)
	cores := int(integer(stringValue(metadata["loginCpuCores"])))
	memoryBytes := integer(stringValue(metadata["loginMemoryKiB"])) * 1024
	var storageBytes int64
	for _, filesystem := range filesystems {
		if available, ok := filesystem["availableKiB"].(int64); ok {
			storageBytes += available * 1024
		}
	}
	loginMetadata := map[string]any{"role": "login", "hostname": name, "architecture": architecture,
		"os": metadata["os"], "kernel": metadata["kernel"], "homeDirectory": metadata["homeDirectory"],
		"workingDirectory": metadata["workingDirectory"], "temporaryDirectory": metadata["temporaryDirectory"],
		"slurmVersion": metadata["slurmVersion"], "containerRuntime": metadata["containerRuntime"], "filesystems": filesystems}
	return &ports.DiscoveredLoginNode{Name: name, Architecture: architecture, CPUCores: cores,
		MemoryBytes: memoryBytes, StorageBytes: storageBytes, Metadata: loginMetadata}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func cleanPartition(value string) string { return strings.TrimSuffix(strings.TrimSpace(value), "*") }
func integer(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}
func csv(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "(null)" || value == "N/A" {
		return []string{}
	}
	return strings.Split(value, ",")
}
func emptyIfNone(value string) string {
	if value == "none" || value == "(null)" {
		return ""
	}
	return value
}
func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

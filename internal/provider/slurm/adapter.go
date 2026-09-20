package slurm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

type Adapter struct {
	executor        runtimecommon.CommandExecutor
	partition       string
	scriptDirectory string
	submitFromStdin bool
	directMu        sync.RWMutex
	directResults   map[string]directResult
	directCancels   map[string]context.CancelFunc
}

type directResult struct {
	output     string
	err        error
	finishedAt float64
}

type Config struct {
	Partition       string
	ScriptDirectory string
	// SubmitFromStdin is required for an SSH-backed adapter: the audit copy of
	// the script stays with the engine while sbatch receives its contents on the
	// remote login node's stdin.
	SubmitFromStdin bool
}

func New(executor runtimecommon.CommandExecutor, partition string) *Adapter {
	return NewWithConfig(executor, Config{Partition: partition, ScriptDirectory: "storage/slurm/scripts"})
}

func NewWithConfig(executor runtimecommon.CommandExecutor, config Config) *Adapter {
	return &Adapter{executor: executor, partition: config.Partition,
		scriptDirectory: config.ScriptDirectory, submitFromStdin: config.SubmitFromStdin,
		directResults: make(map[string]directResult), directCancels: make(map[string]context.CancelFunc)}
}

func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}

func (a *Adapter) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if err := validateSlurmPrerequisites(execution.Activity); err != nil {
		return domain.ActivityHandle{}, err
	}
	if execution.Resource.ExecutionTarget == domain.ExecutionTargetDirect {
		if err := validatePreparation(execution); err != nil {
			return domain.ActivityHandle{}, err
		}
		return a.startDirect(ctx, execution)
	}
	if a.executor == nil {
		return domain.ActivityHandle{}, fmt.Errorf("slurm command executor is required")
	}
	partition, node := a.partition, ""
	if execution.Resource.Type == domain.ResourceHPCPartition && execution.Resource.ProviderID != "" {
		partition = execution.Resource.ProviderID
	}
	if execution.Resource.Type == domain.ResourceHPCMachine && execution.Resource.ProviderID != "" {
		node = execution.Resource.ProviderID
	}
	activity := execution.Activity
	if activity.Command.WorkingDirectory == "" {
		activity.Command.WorkingDirectory = slurmArtifactRoot(execution.Run.ID, activity.ID)
	}
	script, err := batchScript(execution.Run.ID, activity, partition, node)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	scriptPath, err := a.saveScript(execution.Run.ID, activity.ID, ".sbatch", script)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	arguments, input := []string{"--parsable", scriptPath}, []byte(nil)
	if a.submitFromStdin {
		arguments, input = []string{"--parsable"}, []byte(script)
	}
	// Re-check the materialization gate immediately before submission. The
	// workspace manifest is the authoritative proof that every required input
	// was verified; no job may cross the sbatch boundary without it.
	if err := validatePreparation(execution); err != nil {
		return domain.ActivityHandle{}, err
	}
	output, err := a.executor.Run(ctx, "sbatch", arguments, input)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	jobID, err := parseJobID(output)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	submittedAt := runtimecommon.UnixSeconds(time.Now())
	return domain.ActivityHandle{ID: runtimecommon.NewID("activity"), RunID: execution.Run.ID,
		ActivityID: activity.ID, ResourceID: execution.Resource.ID,
		RuntimeID: execution.RuntimeID, ExternalID: jobID,
		Status: domain.HandleStarting, StartedAt: submittedAt,
		Metadata: map[string]any{"executionTarget": string(domain.ExecutionTargetBatch), "scriptPath": scriptPath,
			"logPath":                   slurmLogPath(execution.Run.ID, activity.ID, jobID),
			"sentinelPath":              slurmSentinelPath(execution.Run.ID, activity.ID, jobID),
			"metricsPath":               slurmSentinelPath(execution.Run.ID, activity.ID, jobID) + ".metrics.tsv",
			domain.TimingSubmittedAt:    submittedAt,
			"artifactObservationDriver": "filesystem-diff", "artifactObservationRoot": activity.Command.WorkingDirectory}}, nil
}

func validatePreparation(execution domain.ActivityExecutionContext) error {
	if execution.Preparation == nil {
		return nil
	}
	if err := execution.Preparation.Ready(); err != nil {
		return fmt.Errorf("activity %q is not ready for Slurm: %w", execution.Activity.ID, err)
	}
	return nil
}

func parseJobID(output []byte) (string, error) {
	lines := strings.Split(string(output), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		jobID := strings.Split(line, ";")[0]
		if _, err := strconv.ParseUint(jobID, 10, 64); err == nil {
			return jobID, nil
		}
	}
	return "", fmt.Errorf("invalid Slurm job id in output %q", strings.TrimSpace(string(output)))
}

func (a *Adapter) Inspect(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	if handle.Metadata["executionTarget"] == string(domain.ExecutionTargetDirect) {
		return a.inspectDirect(handle), nil
	}
	output, err := a.executor.Run(ctx, "sh", []string{"-s", "--",
		metadataString(handle, "logPath"), metadataString(handle, "metricsPath"),
		metadataString(handle, "sentinelPath"), handle.ExternalID}, []byte(slurmInspectionScript))
	if err != nil {
		if handle.Metadata == nil {
			handle.Metadata = make(map[string]any)
		}
		handle.Metadata["statusQueryWarning"] = "Slurm inspection unavailable: " + err.Error()
		return handle, nil
	}
	sections := inspectionSections(string(output))
	if len(sections) == 0 {
		// Compatibility for command executors and older tests that return the
		// accounting row directly.
		return applySlurmStatus(handle, string(output)), nil
	}
	handle.Log = sections["LOG"]
	applyMetricSamples(&handle, sections["METRICS"])
	if observed, found := applySentinelStatus(handle, sections["SENTINEL"]); found {
		return observed, nil
	}
	if accounting := strings.TrimSpace(sections["SACCT"]); accounting != "" {
		return applySlurmStatus(handle, accounting), nil
	}
	if queued := strings.TrimSpace(sections["SQUEUE"]); queued != "" {
		return applySlurmStatus(handle, queued), nil
	}
	if state := slurmControlState(sections["SCONTROL"]); state != "" {
		return applySlurmStatus(handle, state+"|"), nil
	}
	if handle.Metadata == nil {
		handle.Metadata = make(map[string]any)
	}
	handle.Metadata["statusQueryWarning"] = "job absent from sentinel, sacct, squeue and scontrol"
	return handle, nil
}

const slurmInspectionScript = `
encode_file() { [ -n "$2" ] && [ -f "$2" ] && base64 < "$2" | tr -d '\n'; }
encode_command() { shift; "$@" 2>/dev/null | base64 | tr -d '\n' || true; }
printf 'LOG='; encode_file LOG "$1"; printf '\n'
printf 'METRICS='; encode_file METRICS "$2"; printf '\n'
printf 'SENTINEL='; encode_file SENTINEL "$3"; printf '\n'
printf 'SACCT='; encode_command SACCT sacct -j "$4" --noheader --parsable2 --format=State,ExitCode; printf '\n'
printf 'SQUEUE='; encode_command SQUEUE squeue --noheader --jobs "$4" --format=%T; printf '\n'
printf 'SCONTROL='; encode_command SCONTROL scontrol show job "$4" --oneliner; printf '\n'
`

func metadataString(handle domain.ActivityHandle, key string) string {
	value, _ := handle.Metadata[key].(string)
	return value
}

func inspectionSections(payload string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(payload, "\n") {
		key, encoded, found := strings.Cut(line, "=")
		if !found || (key != "LOG" && key != "METRICS" && key != "SENTINEL" && key != "SACCT" && key != "SQUEUE" && key != "SCONTROL") {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil {
			result[key] = string(decoded)
		}
	}
	return result
}

func (a *Adapter) sentinelStatus(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, bool) {
	path, ok := handle.Metadata["sentinelPath"].(string)
	if !ok || path == "" {
		return handle, false
	}
	payload, err := a.executor.Run(ctx, "cat", []string{path}, nil)
	if err != nil {
		return handle, false
	}
	return applySentinelStatus(handle, string(payload))
}

func applySentinelStatus(handle domain.ActivityHandle, payload string) (domain.ActivityHandle, bool) {
	values := sentinelValues(payload)
	if startedAt, err := strconv.ParseFloat(values["started_at"], 64); err == nil && startedAt > 0 {
		handle.StartedAt = startedAt
	}
	if containerStartedAt, err := strconv.ParseFloat(values["container_started_at"], 64); err == nil && containerStartedAt > 0 {
		if handle.Metadata == nil {
			handle.Metadata = make(map[string]any)
		}
		handle.Metadata[domain.TimingContainerStartedAt] = containerStartedAt
	}
	if node := strings.TrimSpace(values["allocated_node"]); node != "" {
		if handle.Metadata == nil {
			handle.Metadata = make(map[string]any)
		}
		handle.Metadata["allocatedNode"] = node
	}
	switch values["state"] {
	case "running":
		handle.Status = domain.HandleRunning
	case "completed":
		handle.Status = domain.HandleCompleted
		handle.FinishedAt = sentinelFinishedAt(values)
	case "failed":
		handle.Status = domain.HandleFailed
		handle.FinishedAt = sentinelFinishedAt(values)
		handle.Failure = "Slurm job exited with code " + values["exit_code"]
	default:
		return handle, false
	}
	if code, err := strconv.Atoi(values["exit_code"]); err == nil {
		handle.ExitCode = &code
	}
	if handle.Status == domain.HandleCompleted || handle.Status == domain.HandleFailed {
		handle.Artifacts = slurmArtifacts(handle, values)
	}
	return handle, true
}

func sentinelFinishedAt(values map[string]string) float64 {
	if finishedAt, err := strconv.ParseFloat(values["finished_at"], 64); err == nil && finishedAt > 0 {
		return finishedAt
	}
	return runtimecommon.UnixSeconds(time.Now())
}

func slurmArtifacts(handle domain.ActivityHandle, values map[string]string) *domain.ArtifactManifest {
	root := values["artifact_root"]
	if root == "" {
		root, _ = handle.Metadata["artifactObservationRoot"].(string)
	}
	manifest := &domain.ArtifactManifest{SchemaVersion: 1, RunID: handle.RunID, ActivityID: handle.ActivityID,
		Attempt: 1, Runtime: handle.RuntimeID, Root: root, StartedAt: handle.StartedAt, FinishedAt: handle.FinishedAt,
		Files: make([]domain.ArtifactObservation, 0)}
	if handle.ExitCode != nil {
		manifest.ExitCode = *handle.ExitCode
	}
	manifest.InitialSnapshot = slurmSnapshotEntries(values, "initial.")
	manifest.FinalSnapshot = slurmSnapshotEntries(values, "final.")
	if len(manifest.InitialSnapshot) > 0 || len(manifest.FinalSnapshot) > 0 {
		populateSlurmArtifactDelta(manifest)
	}
	for key, value := range values {
		if !strings.HasPrefix(key, "artifact.") {
			continue
		}
		parts := strings.SplitN(value, "|", 3)
		if len(parts) != 3 {
			continue
		}
		path, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			continue
		}
		size, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		manifest.Files = append(manifest.Files, domain.ArtifactObservation{Path: string(path), Change: domain.ArtifactCreated, SizeBytes: size, Checksum: "sha256:" + strings.TrimSpace(parts[2])})
		manifest.Summary.CreatedFiles++
		manifest.Summary.OutputBytes += size
	}
	if len(manifest.FinalSnapshot) == 0 {
		manifest.Summary.FinalFiles = len(manifest.Files)
	}
	phase := "completed"
	if manifest.ExitCode != 0 {
		phase = "failed"
	}
	duration := handle.FinishedAt - handle.StartedAt
	if duration < 0 {
		duration = 0
	}
	manifest.Phases = []domain.LifecycleObservation{{Phase: "execution", Status: phase, StartedAt: handle.StartedAt, FinishedAt: handle.FinishedAt, DurationSeconds: duration}}
	return manifest
}

func slurmSnapshotEntries(values map[string]string, prefix string) []domain.ArtifactSnapshotEntry {
	entries := make([]domain.ArtifactSnapshotEntry, 0)
	for key, value := range values {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		parts := strings.SplitN(value, "|", 3)
		if len(parts) != 3 {
			continue
		}
		path, pathErr := base64.StdEncoding.DecodeString(parts[0])
		size, sizeErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if pathErr != nil || sizeErr != nil {
			continue
		}
		entries = append(entries, domain.ArtifactSnapshotEntry{Path: string(path), SizeBytes: size,
			Checksum: "sha256:" + strings.TrimSpace(parts[2])})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func populateSlurmArtifactDelta(manifest *domain.ArtifactManifest) {
	initial := make(map[string]domain.ArtifactSnapshotEntry, len(manifest.InitialSnapshot))
	final := make(map[string]domain.ArtifactSnapshotEntry, len(manifest.FinalSnapshot))
	paths := make([]string, 0, len(manifest.InitialSnapshot)+len(manifest.FinalSnapshot))
	seen := make(map[string]bool)
	for _, entry := range manifest.InitialSnapshot {
		initial[entry.Path] = entry
		seen[entry.Path] = true
		paths = append(paths, entry.Path)
	}
	for _, entry := range manifest.FinalSnapshot {
		final[entry.Path] = entry
		if !seen[entry.Path] {
			paths = append(paths, entry.Path)
		}
	}
	sort.Strings(paths)
	manifest.Summary.InitialFiles = len(initial)
	manifest.Summary.FinalFiles = len(final)
	for _, path := range paths {
		before, hadBefore := initial[path]
		after, hasAfter := final[path]
		change := domain.ArtifactCreated
		switch {
		case !hadBefore:
			manifest.Summary.CreatedFiles++
		case !hasAfter:
			change = domain.ArtifactDeleted
			manifest.Summary.DeletedFiles++
			after = before
		case before.Checksum == after.Checksum && before.SizeBytes == after.SizeBytes:
			continue
		default:
			change = domain.ArtifactModified
			manifest.Summary.ModifiedFiles++
		}
		manifest.Files = append(manifest.Files, domain.ArtifactObservation{Path: path, Change: change,
			SizeBytes: after.SizeBytes, Checksum: after.Checksum})
		if change != domain.ArtifactDeleted {
			manifest.Summary.OutputBytes += after.SizeBytes
		}
	}
}

func (a *Adapter) fallbackStatus(ctx context.Context, handle domain.ActivityHandle, sacctErr error) (domain.ActivityHandle, error) {
	if output, err := a.executor.Run(ctx, "squeue", []string{"--noheader", "--jobs", handle.ExternalID, "--format=%T"}, nil); err == nil {
		if strings.TrimSpace(string(output)) != "" {
			return applySlurmStatus(handle, string(output)), nil
		}
	}
	if output, err := a.executor.Run(ctx, "scontrol", []string{"show", "job", handle.ExternalID, "--oneliner"}, nil); err == nil {
		if state := slurmControlState(string(output)); state != "" {
			return applySlurmStatus(handle, state+"|"), nil
		}
	}
	if handle.Metadata == nil {
		handle.Metadata = make(map[string]any)
	}
	// sacct depends on slurmdbd and may be unavailable while the scheduler is
	// healthy. Absence from squeue/scontrol is not proof that the job failed:
	// completed jobs can disappear from controller memory before accounting is
	// restored. Keep polling the AkôFlow sentinel instead of creating a false
	// terminal failure.
	handle.Metadata["statusQueryWarning"] = "sacct unavailable: " + sacctErr.Error()
	return handle, nil
}

func applySlurmStatus(handle domain.ActivityHandle, output string) domain.ActivityHandle {
	line := strings.TrimSpace(strings.Split(output, "\n")[0])
	fields := strings.Split(line, "|")
	if len(fields) == 0 || fields[0] == "" {
		return handle
	}
	state := strings.Split(fields[0], "+")[0]
	switch state {
	case "PENDING", "CONFIGURING":
		handle.Status = domain.HandleStarting
	case "RUNNING", "COMPLETING":
		handle.Status = domain.HandleRunning
	case "COMPLETED":
		handle.Status = domain.HandleCompleted
	case "CANCELLED":
		handle.Status = domain.HandleStopped
	default:
		handle.Status = domain.HandleFailed
		handle.Failure = "Slurm state: " + state
	}
	if len(fields) > 1 {
		codeText := strings.Split(fields[1], ":")[0]
		if code, parseErr := strconv.Atoi(codeText); parseErr == nil {
			handle.ExitCode = &code
		}
	}
	if handle.Status == domain.HandleCompleted || handle.Status == domain.HandleFailed || handle.Status == domain.HandleStopped {
		handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
	}
	return handle
}

func sentinelValues(payload string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(payload, "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			if key == "artifact" || key == "initial" || key == "final" {
				key = "artifact." + strconv.Itoa(len(values))
				if strings.HasPrefix(line, "initial=") {
					key = "initial." + strconv.Itoa(len(values))
				} else if strings.HasPrefix(line, "final=") {
					key = "final." + strconv.Itoa(len(values))
				}
			}
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}

func slurmControlState(payload string) string {
	for _, field := range strings.Fields(payload) {
		if value, found := strings.CutPrefix(field, "JobState="); found {
			return value
		}
	}
	return ""
}

func (a *Adapter) Stop(ctx context.Context, handle domain.ActivityHandle) error {
	if handle.Metadata["executionTarget"] == string(domain.ExecutionTargetDirect) {
		a.directMu.Lock()
		cancel := a.directCancels[handle.ID]
		a.directMu.Unlock()
		if cancel == nil {
			return fmt.Errorf("direct activity %q is not running", handle.ID)
		}
		cancel()
		return nil
	}
	_, err := a.executor.Run(ctx, "scancel", []string{handle.ExternalID}, nil)
	return err
}

func (a *Adapter) startDirect(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if a.executor == nil {
		return domain.ActivityHandle{}, fmt.Errorf("slurm command executor is required")
	}
	activity := execution.Activity
	script, err := directScript(activity)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	scriptPath, err := a.saveScript(execution.Run.ID, activity.ID, ".sh", script)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	startedAt := runtimecommon.UnixSeconds(time.Now())
	handle := domain.ActivityHandle{ID: execution.Run.ID + ":" + activity.ID, RunID: execution.Run.ID,
		ActivityID: activity.ID, ResourceID: execution.Resource.ID, RuntimeID: execution.RuntimeID,
		Status: domain.HandleRunning, StartedAt: startedAt, Metadata: map[string]any{
			"executionTarget": string(domain.ExecutionTargetDirect), "slurmSubmission": "login-node",
			"scriptPath": scriptPath, domain.TimingSubmittedAt: startedAt,
		}}
	runContext, cancel := context.WithCancel(ctx)
	a.directMu.Lock()
	a.directCancels[handle.ID] = cancel
	a.directMu.Unlock()
	go func() {
		output, runErr := a.executor.Run(runContext, "sh", []string{"-s"}, []byte(script))
		a.directMu.Lock()
		a.directResults[handle.ID] = directResult{output: string(output), err: runErr, finishedAt: runtimecommon.UnixSeconds(time.Now())}
		delete(a.directCancels, handle.ID)
		a.directMu.Unlock()
	}()
	return handle, nil
}

func (a *Adapter) inspectDirect(handle domain.ActivityHandle) domain.ActivityHandle {
	a.directMu.RLock()
	result, done := a.directResults[handle.ID]
	a.directMu.RUnlock()
	if !done {
		return handle
	}
	handle.Log = result.output
	handle.FinishedAt = result.finishedAt
	if result.err != nil {
		handle.Status = domain.HandleFailed
		handle.Failure = result.err.Error()
		return handle
	}
	handle.Status = domain.HandleCompleted
	return handle
}

func (a *Adapter) saveScript(runID, activityID, extension, content string) (string, error) {
	if a.scriptDirectory == "" {
		return "", fmt.Errorf("slurm script directory is required")
	}
	directory := filepath.Join(a.scriptDirectory, shellToken(runID), shellToken(activityID))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create Slurm script directory: %w", err)
	}
	path := filepath.Join(directory, fmt.Sprintf("%d%s", time.Now().UnixNano(), extension))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("save Slurm script: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve Slurm script path: %w", err)
	}
	return abs, nil
}

func directScript(activity domain.Activity) (string, error) {
	if activity.Command.Entrypoint == "" {
		return "", fmt.Errorf("activity entrypoint is required")
	}
	var script strings.Builder
	script.WriteString("#!/bin/sh\nset -eu\n")
	if err := writeActivityCommand(&script, activity, "", ""); err != nil {
		return "", err
	}
	return script.String(), nil
}

func batchScript(runID string, activity domain.Activity, partition, node string) (string, error) {
	if activity.Command.Entrypoint == "" {
		return "", fmt.Errorf("activity entrypoint is required")
	}
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("#SBATCH --job-name=")
	script.WriteString(shellToken("akoflow-" + activity.ID))
	script.WriteByte('\n')
	script.WriteString("#SBATCH --output=")
	script.WriteString(slurmLogPath(runID, activity.ID, "%j"))
	script.WriteByte('\n')
	if partition != "" {
		script.WriteString("#SBATCH --partition=")
		script.WriteString(shellToken(partition))
		script.WriteByte('\n')
	}
	if node != "" {
		script.WriteString("#SBATCH --nodelist=")
		script.WriteString(shellToken(node))
		script.WriteByte('\n')
	}
	if activity.Resources.CPU > 0 {
		script.WriteString(fmt.Sprintf("#SBATCH --cpus-per-task=%d\n", int(activity.Resources.CPU+0.999)))
	}
	if activity.Resources.MemoryBytes > 0 {
		script.WriteString(fmt.Sprintf("#SBATCH --mem=%dM\n", (activity.Resources.MemoryBytes+(1<<20)-1)/(1<<20)))
	}
	sentinelPrefix := slurmSentinelPrefix(runID, activity.ID)
	// Keep the sentinel in the submission directory. The activity command may
	// change into its workspace, and a relative sentinel path would otherwise
	// leave the initial `running` file behind while writing completion there.
	script.WriteString("sentinel_dir=$(pwd)\nsentinel=$(printf '%s/%s' \"$sentinel_dir\" ")
	script.WriteString(shellQuote(sentinelPrefix))
	script.WriteString(")\nsentinel=\"${sentinel}${SLURM_JOB_ID}.status\"\n")
	script.WriteString("artifact_root=")
	script.WriteString(shellQuote(activity.Command.WorkingDirectory))
	script.WriteString("\ncontainer_start_marker=\"${sentinel}.container-started\"\nrm -f \"$container_start_marker\"\nmkdir -p \"$artifact_root\"\nartifact_root=$(cd \"$artifact_root\" && pwd -P)\nartifact_before=$(mktemp)\nartifact_after=$(mktemp)\nseed_links=$(mktemp)\nseed_list=$(mktemp)\n")
	script.WriteString(`snapshot_artifacts() { destination=$1; : > "$destination"; find "$artifact_root" -type f -exec sh -c '
root=$1; destination=$2; shift 2
for file do relative=${file#"$root"/}; size=$(wc -c < "$file") || exit 1; checksum=$(sha256sum "$file" | awk "{print \$1}") || exit 1; encoded=$(printf "%s" "$relative" | base64 | tr -d "\\n"); printf "%s|%s|%s\\n" "$encoded" "$size" "$checksum" >> "$destination"; done
' snapshot "$artifact_root" "$destination" {} +; sort -o "$destination" "$destination"; }
`)
	script.WriteString("started_at=$(date +%s.%N)\nallocated_node=$(hostname -s)\ncontainer_epoch_anchor=$(date +%s.%N)\ncontainer_uptime_anchor=$(awk '{print $1}' /proc/uptime)\nprintf 'state=running\\nstarted_at=%s\\nallocated_node=%s\\nartifact_root=%s\\n' \"$started_at\" \"$allocated_node\" \"$artifact_root\" > \"$sentinel\"\n")
	if interval := activity.Command.Environment["AKOFLOW_METRIC_INTERVAL_SECONDS"]; interval != "" {
		script.WriteString("AKOFLOW_METRIC_INTERVAL_SECONDS=")
		script.WriteString(shellQuote(interval))
		script.WriteByte('\n')
	}
	script.WriteString(metricSamplerScript)
	// The activity runs with set -e, but sentinel publication must remain
	// best-effort even when final metric sampling or artifact inspection fails.
	// Capture the activity result first, then disable errexit inside the trap.
	script.WriteString("finish() { code=$?; set +e; state=completed; [ \"$code\" -eq 0 ] || state=failed; ")
	script.WriteString("[ -z \"$metric_pid\" ] || { kill \"$metric_pid\" 2>/dev/null || true; wait \"$metric_pid\" 2>/dev/null || true; }; ")
	script.WriteString("[ -z \"$metric_pid\" ] || sample_metrics; ")
	script.WriteString("container_started_at=0; if [ -f \"$container_start_marker\" ]; then container_uptime=$(cat \"$container_start_marker\"); container_started_at=$(awk -v epoch=\"$container_epoch_anchor\" -v anchor=\"$container_uptime_anchor\" -v current=\"$container_uptime\" 'BEGIN { printf \"%.9f\", epoch + current - anchor }'); fi; ")
	script.WriteString("while IFS= read -r seed_link; do [ -L \"$seed_link\" ] && rm -f -- \"$seed_link\"; done < \"$seed_links\"; ")
	script.WriteString("snapshot_artifacts \"$artifact_after\" || true; ")
	script.WriteString("finished_at=$(date +%s.%N); { printf 'state=%s\\nexit_code=%s\\nstarted_at=%s\\nfinished_at=%s\\nallocated_node=%s\\ncontainer_started_at=%s\\nartifact_root=%s\\n' \"$state\" \"$code\" \"$started_at\" \"$finished_at\" \"$allocated_node\" \"$container_started_at\" \"$artifact_root\"; ")
	script.WriteString("while IFS= read -r entry; do printf 'initial=%s\\n' \"$entry\"; done < \"$artifact_before\"; ")
	script.WriteString("while IFS= read -r entry; do printf 'final=%s\\n' \"$entry\"; done < \"$artifact_after\"; ")
	script.WriteString("} > \"$sentinel\" || true; rm -f \"$artifact_before\" \"$artifact_after\" \"$container_start_marker\" \"$seed_links\" \"$seed_list\"; exit \"$code\"; }\n")
	script.WriteString("trap finish EXIT\nset -eu\n")
	image, err := slurmExecutable(activity.Command)
	if err != nil {
		return "", err
	}
	if image != "" {
		writeSharedImageSeedPreparation(&script, image)
	}
	script.WriteString("snapshot_artifacts \"$artifact_before\"\n")
	if err := writeActivityCommand(&script, activity, "$container_start_marker", "$artifact_root"); err != nil {
		return "", err
	}
	return script.String(), nil
}

func writeSharedImageSeedPreparation(script *strings.Builder, image string) {
	script.WriteString("container_runtime=$(command -v apptainer || command -v singularity)\n")
	script.WriteString("image=")
	script.WriteString(shellQuote(image))
	script.WriteByte('\n')
	script.WriteString("if \"$container_runtime\" exec \"$image\" test -d /akoflow-wfa-shared; then\n")
	script.WriteString("  \"$container_runtime\" exec \"$image\" find /akoflow-wfa-shared -mindepth 1 \\( -type f -o -type l \\) -print > \"$seed_list\" || { echo 'AkôFlow shared seed preparation failed while listing image inputs.' >&2; exit 70; }\n")
	script.WriteString("  while IFS= read -r seed_source; do\n")
	script.WriteString("    seed_relative=${seed_source#/akoflow-wfa-shared/}\n")
	script.WriteString("    seed_target=$artifact_root/$seed_relative\n")
	script.WriteString("    if [ -e \"$seed_target\" ] || [ -L \"$seed_target\" ]; then continue; fi\n")
	script.WriteString("    mkdir -p -- \"$(dirname \"$seed_target\")\" || { echo \"AkôFlow shared seed preparation could not create the directory for $seed_relative.\" >&2; exit 70; }\n")
	script.WriteString("    ln -s -- \"$seed_source\" \"$seed_target\" || { echo \"AkôFlow shared seed preparation could not link $seed_relative.\" >&2; exit 70; }\n")
	script.WriteString("    printf '%s\\n' \"$seed_target\" >> \"$seed_links\"\n")
	script.WriteString("  done < \"$seed_list\"\n")
	script.WriteString("fi\n")
}

func slurmLogPath(runID, activityID, jobID string) string {
	return slurmSentinelPrefix(runID, activityID) + jobID + ".log"
}

func slurmSentinelPath(runID, activityID, jobID string) string {
	return slurmSentinelPrefix(runID, activityID) + jobID + ".status"
}

func slurmSentinelPrefix(runID, activityID string) string {
	return "akoflow-" + shellToken(runID) + "-" + shellToken(activityID) + "-"
}

func slurmArtifactRoot(runID, activityID string) string {
	return "akoflow-workspaces/" + shellToken(runID) + "/" + shellToken(activityID)
}

func writeActivityCommand(script *strings.Builder, activity domain.Activity, containerStartMarker, containerBindRoot string) error {
	for key, value := range activity.Command.Environment {
		script.WriteString("export ")
		script.WriteString(shellToken(key))
		script.WriteByte('=')
		script.WriteString(shellQuote(value))
		script.WriteByte('\n')
	}
	if activity.Command.WorkingDirectory != "" {
		script.WriteString("cd ")
		if containerBindRoot != "" {
			script.WriteString("\"")
			script.WriteString(containerBindRoot)
			script.WriteString("\"")
		} else {
			script.WriteString(shellQuote(activity.Command.WorkingDirectory))
		}
		script.WriteByte('\n')
	}
	image, err := slurmExecutable(activity.Command)
	if err != nil {
		return err
	}
	if image != "" {
		script.WriteString("${container_runtime:-singularity} exec ")
		if containerBindRoot != "" {
			script.WriteString("--bind \"")
			script.WriteString(containerBindRoot)
			script.WriteString(":")
			script.WriteString(containerBindRoot)
			script.WriteString("\" ")
		}
		script.WriteString(shellQuote(image))
		if containerStartMarker != "" {
			script.WriteString(" /bin/sh -c ")
			// BusyBox, used by many OCI images, does not support date's %N format.
			// /proc/uptime is shared with the node and retains a portable fractional
			// clock; the outer batch script converts it back to epoch time.
			script.WriteString(shellQuote(`awk '{print $1}' /proc/uptime > "$1"; shift; exec "$@"`))
			script.WriteString(" sh ")
			script.WriteString(containerStartMarker)
			script.WriteByte(' ')
		} else {
			script.WriteByte(' ')
		}
	}
	script.WriteString(shellQuote(activity.Command.Entrypoint))
	for _, argument := range activity.Command.Arguments {
		script.WriteByte(' ')
		script.WriteString(shellQuote(argument))
	}
	script.WriteByte('\n')
	return nil
}

func validateSlurmPrerequisites(activity domain.Activity) error {
	_, err := slurmExecutable(activity.Command)
	if err != nil {
		return fmt.Errorf("activity %q is not ready for Slurm: %w", activity.ID, err)
	}
	return nil
}

// slurmExecutable never lets a Slurm node fetch an executable itself. The
// control-plane gateway must materialize and transfer every non-shared SIF
// before submission.
func slurmExecutable(command domain.ActivityCommand) (string, error) {
	if resolved := command.ResolvedExecutable; resolved != nil {
		path := resolved.LocalPath
		if path == "" {
			path = resolved.RemotePath
		}
		if path == "" || !resolved.MaterializationDone {
			return "", fmt.Errorf("executable materialization is not committed")
		}
		return path, nil
	}
	ref := command.EffectiveExecutable()
	if ref == nil {
		return "", nil
	}
	// A legacy SIF/path is already a location. Legacy OCI references are not.
	image := strings.TrimSpace(command.Image)
	if ref.Source.Type == domain.ExecutableSourceRemoteFile && ref.Source.Path != "" && ref.Delivery.Strategy == domain.DeliveryUseInPlace {
		return ref.Source.Path, nil
	}
	if image != "" && (strings.HasPrefix(image, "/") || strings.HasPrefix(image, "./") || strings.HasSuffix(image, ".sif")) {
		return image, nil
	}
	return "", fmt.Errorf("executable %q requires artifact materialization before Slurm submission", ref.Source.Reference)
}

func shellToken(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r) {
			return r
		}
		return '-'
	}, value)
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

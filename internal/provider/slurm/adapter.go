package slurm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/local"
)

type Adapter struct {
	executor        runtimecommon.CommandExecutor
	partition       string
	direct          *local.Adapter
	scriptDirectory string
	submitFromStdin bool
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
	return &Adapter{executor: executor, partition: config.Partition, direct: local.New(),
		scriptDirectory: config.ScriptDirectory, submitFromStdin: config.SubmitFromStdin}
}

func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}

func (a *Adapter) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if execution.Preparation != nil {
		if err := execution.Preparation.Ready(); err != nil {
			return domain.ActivityHandle{}, fmt.Errorf("activity %q is not ready for Slurm: %w", execution.Activity.ID, err)
		}
	}
	if err := validateSlurmPrerequisites(execution.Activity); err != nil {
		return domain.ActivityHandle{}, err
	}
	if execution.Resource.ExecutionTarget == domain.ExecutionTargetDirect {
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
			domain.TimingSubmittedAt:    submittedAt,
			"artifactObservationDriver": "filesystem-diff", "artifactObservationRoot": activity.Command.WorkingDirectory}}, nil
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
		return a.direct.Inspect(ctx, handle)
	}
	if logPath, ok := handle.Metadata["logPath"].(string); ok && logPath != "" {
		if log, logErr := a.executor.Run(ctx, "cat", []string{logPath}, nil); logErr == nil {
			handle.Log = string(log)
		}
	}
	if observed, found := a.sentinelStatus(ctx, handle); found {
		return observed, nil
	}
	output, err := a.executor.Run(ctx, "sacct", []string{"-j", handle.ExternalID,
		"--noheader", "--parsable2", "--format=State,ExitCode"}, nil)
	if err != nil {
		return a.fallbackStatus(ctx, handle, err)
	}
	return applySlurmStatus(handle, string(output)), nil
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
	values := sentinelValues(string(payload))
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
		handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
	case "failed":
		handle.Status = domain.HandleFailed
		handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
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
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		manifest.Files = append(manifest.Files, domain.ArtifactObservation{Path: string(path), Change: domain.ArtifactCreated, SizeBytes: size, Checksum: "sha256:" + parts[2]})
		manifest.Summary.CreatedFiles++
		manifest.Summary.OutputBytes += size
	}
	manifest.Summary.FinalFiles = len(manifest.Files)
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
	handle.Status = domain.HandleFailed
	handle.Failure = "query Slurm job status: " + sacctErr.Error()
	handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
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
			if key == "artifact" {
				key = "artifact." + strconv.Itoa(len(values))
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
		return a.direct.Stop(ctx, handle)
	}
	_, err := a.executor.Run(ctx, "scancel", []string{handle.ExternalID}, nil)
	return err
}

func (a *Adapter) startDirect(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	activity := execution.Activity
	script, err := directScript(activity)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	scriptPath, err := a.saveScript(execution.Run.ID, activity.ID, ".sh", script)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	execution.Activity.Command = domain.ActivityCommand{Entrypoint: "/bin/sh", Arguments: []string{scriptPath}, WorkingDirectory: activity.Command.WorkingDirectory}
	handle, err := a.direct.Start(ctx, execution)
	if err != nil {
		return handle, err
	}
	if handle.Metadata == nil {
		handle.Metadata = make(map[string]any)
	}
	handle.Metadata["executionTarget"] = string(domain.ExecutionTargetDirect)
	handle.Metadata["slurmSubmission"] = "login-node"
	handle.Metadata["scriptPath"] = scriptPath
	return handle, nil
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
	if err := writeActivityCommand(&script, activity, ""); err != nil {
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
	script.WriteString("\ncontainer_start_marker=\"${sentinel}.container-started\"\nrm -f \"$container_start_marker\"\nmkdir -p \"$artifact_root\"\nartifact_before=$(mktemp)\nfind \"$artifact_root\" -type f -print 2>/dev/null | sort > \"$artifact_before\"\n")
	script.WriteString("started_at=$(date +%s.%N)\nallocated_node=$(hostname -s)\ncontainer_epoch_anchor=$(date +%s.%N)\ncontainer_uptime_anchor=$(awk '{print $1}' /proc/uptime)\nprintf 'state=running\\nstarted_at=%s\\nallocated_node=%s\\nartifact_root=%s\\n' \"$started_at\" \"$allocated_node\" \"$artifact_root\" > \"$sentinel\"\n")
	script.WriteString("finish() { code=$?; state=completed; [ \"$code\" -eq 0 ] || state=failed; ")
	script.WriteString("container_started_at=0; if [ -f \"$container_start_marker\" ]; then container_uptime=$(cat \"$container_start_marker\"); container_started_at=$(awk -v epoch=\"$container_epoch_anchor\" -v anchor=\"$container_uptime_anchor\" -v current=\"$container_uptime\" 'BEGIN { printf \"%.9f\", epoch + current - anchor }'); fi; ")
	script.WriteString("{ printf 'state=%s\\nexit_code=%s\\nstarted_at=%s\\nallocated_node=%s\\ncontainer_started_at=%s\\nartifact_root=%s\\n' \"$state\" \"$code\" \"$started_at\" \"$allocated_node\" \"$container_started_at\" \"$artifact_root\"; ")
	script.WriteString("find \"$artifact_root\" -type f -print 2>/dev/null | sort | comm -13 \"$artifact_before\" - | ")
	script.WriteString("while IFS= read -r file; do relative=${file#\"$artifact_root\"/}; ")
	script.WriteString("size=$(wc -c < \"$file\" 2>/dev/null) || continue; ")
	script.WriteString("checksum=$(sha256sum \"$file\" 2>/dev/null | awk '{print $1}') || continue; ")
	script.WriteString("encoded=$(printf '%s' \"$relative\" | base64 | tr -d '\\n'); ")
	script.WriteString("printf 'artifact=%s|%s|%s\\n' \"$encoded\" \"$size\" \"$checksum\"; done; ")
	script.WriteString("} > \"$sentinel\" || true; rm -f \"$artifact_before\" \"$container_start_marker\"; exit \"$code\"; }\n")
	script.WriteString("trap finish EXIT\nset -eu\n")
	if err := writeActivityCommand(&script, activity, "$container_start_marker"); err != nil {
		return "", err
	}
	return script.String(), nil
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

func writeActivityCommand(script *strings.Builder, activity domain.Activity, containerStartMarker string) error {
	for key, value := range activity.Command.Environment {
		script.WriteString("export ")
		script.WriteString(shellToken(key))
		script.WriteByte('=')
		script.WriteString(shellQuote(value))
		script.WriteByte('\n')
	}
	if activity.Command.WorkingDirectory != "" {
		script.WriteString("cd ")
		script.WriteString(shellQuote(activity.Command.WorkingDirectory))
		script.WriteByte('\n')
	}
	image, err := slurmExecutable(activity.Command)
	if err != nil {
		return err
	}
	if image != "" {
		script.WriteString("singularity exec ")
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

// Package remote implements direct Docker execution on an SSH-connected host.
// It intentionally has no scheduler dependency.
package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/telemetry"
)

type Factory struct{ Executor runtimecommon.CommandExecutor }

func (Factory) Driver() domain.RuntimeDriver { return domain.RuntimeDriverSSH }

func (f Factory) Build(_ domain.EnvironmentRuntime, connection domain.EnvironmentConnection) (ports.RuntimeAdapter, error) {
	if connection.Type != domain.ConnectionSSH || !directDocker(connection) {
		return nil, fmt.Errorf("SSH runtime requires a direct Docker SSH connection")
	}
	executor := f.Executor
	if executor == nil {
		executor = runtimecommon.OSCommandExecutor{}
	}
	return &Adapter{executor: runtimecommon.NewSSHCommandExecutor(executor, connection)}, nil
}

type Adapter struct{ executor runtimecommon.CommandExecutor }

type remoteArtifactFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Modified int64  `json:"modified"`
}

func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}

func (a *Adapter) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if execution.Preparation != nil {
		if err := execution.Preparation.Ready(); err != nil {
			return domain.ActivityHandle{}, err
		}
	}
	activity := execution.Activity
	if strings.TrimSpace(activity.Command.Image) == "" {
		return domain.ActivityHandle{}, fmt.Errorf("activity image is required for remote Docker execution")
	}
	name := "akoflow-" + safeName(execution.Run.ID) + "-" + safeName(activity.ID)
	workingDirectory := strings.TrimSpace(activity.Command.WorkingDirectory)
	if workingDirectory == "" {
		workingDirectory = "/akoflow/workspace/runs/" + safeName(execution.Run.ID) + "/" + safeName(activity.ID)
	}
	if _, err := a.executor.Run(ctx, "mkdir", []string{"-p", workingDirectory}, nil); err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("prepare remote workspace: %w", err)
	}
	before, err := a.snapshot(ctx, workingDirectory)
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("snapshot remote workspace: %w", err)
	}
	beforeJSON, _ := json.Marshal(before)
	args := []string{"run", "--detach", "--name", name, "--label", "akoflow.run=" + execution.Run.ID}
	for key, value := range activity.Command.Environment {
		args = append(args, "--env", key+"="+value)
	}
	if activity.Resources.CPU > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(activity.Resources.CPU, 'f', -1, 64))
	}
	if activity.Resources.MemoryBytes > 0 {
		args = append(args, "--memory", strconv.FormatInt(activity.Resources.MemoryBytes, 10))
	}
	args = append(args, "--volume", workingDirectory+":"+workingDirectory, "--workdir", workingDirectory)
	args = append(args, activity.Command.Image)
	if seed, _ := activity.Metadata["workspaceSeedPath"].(string); seed != "" {
		args = append(args, "sh", "-c", `cp -an "$1"/. "$2"/ && cd "$2" && shift 2 && exec "$@"`, "akoflow-seed", seed, workingDirectory)
	}
	if activity.Command.Entrypoint != "" {
		args = append(args, activity.Command.Entrypoint)
	}
	args = append(args, activity.Command.Arguments...)
	output, err := a.executor.Run(ctx, "docker", args, nil)
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("start remote Docker container: %w", err)
	}
	// The SSH executor combines stdout and stderr. On a first run Docker may
	// print image-pull progress before the container ID; only the final line is
	// accepted by docker inspect, logs and rm.
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	containerID := strings.TrimSpace(lines[len(lines)-1])
	if containerID == "" {
		return domain.ActivityHandle{}, fmt.Errorf("start remote Docker container: missing container ID")
	}
	now := runtimecommon.UnixSeconds(time.Now())
	handle := domain.ActivityHandle{ID: runtimecommon.NewID("activity"), RunID: execution.Run.ID, ActivityID: activity.ID,
		ResourceID: execution.Resource.ID, RuntimeID: execution.RuntimeID, ExternalID: containerID,
		Status: domain.HandleStarting, StartedAt: now, Metadata: map[string]any{
			"containerName": name, "executionTarget": "remote-docker", "artifactObservationRoot": workingDirectory,
			"artifactObservationBefore": string(beforeJSON),
		}}
	telemetry.ObserveDocker(ctx, &handle, containerID, a.executor.Run)
	return handle, nil
}

func (a *Adapter) Inspect(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	output, err := a.executor.Run(ctx, "docker", []string{"inspect", "--format", "{{.State.Status}}|{{.State.ExitCode}}", handle.ExternalID}, nil)
	if err != nil {
		return handle, err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	switch parts[0] {
	case "running", "created":
		handle.Status = domain.HandleRunning
		telemetry.ObserveDocker(ctx, &handle, handle.ExternalID, a.executor.Run)
	case "exited":
		code, _ := strconv.Atoi(parts[1])
		handle.ExitCode = &code
		handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
		if code == 0 {
			handle.Status = domain.HandleCompleted
		} else {
			handle.Status = domain.HandleFailed
			handle.Failure = "remote Docker container exited with code " + strconv.Itoa(code)
		}
		root, _ := handle.Metadata["artifactObservationRoot"].(string)
		if root != "" {
			artifacts, observeErr := a.collectArtifacts(ctx, handle, code)
			if observeErr == nil {
				handle.Artifacts = artifacts
			} else {
				if handle.Metadata == nil {
					handle.Metadata = map[string]any{}
				}
				handle.Metadata["artifactObservationError"] = observeErr.Error()
				if code == 0 {
					handle.Status = domain.HandleFailed
					handle.Failure = "observe remote workspace: " + observeErr.Error()
				}
			}
		}
	default:
		handle.Status = domain.HandleFailed
		handle.Failure = "remote Docker container state: " + parts[0]
	}
	if log, logErr := a.executor.Run(ctx, "docker", []string{"logs", handle.ExternalID}, nil); logErr == nil {
		handle.Log = string(log)
	}
	return handle, nil
}

func (a *Adapter) snapshot(ctx context.Context, root string) ([]remoteArtifactFile, error) {
	const script = `import hashlib,json,os,sys
root=sys.argv[1]
result=[]
for base,dirs,files in os.walk(root):
  dirs.sort(); files.sort()
  for name in files:
    full=os.path.join(base,name)
    if not os.path.isfile(full): continue
    h=hashlib.sha256()
    with open(full,'rb') as stream:
      for chunk in iter(lambda: stream.read(1024*1024),b''): h.update(chunk)
    stat=os.stat(full)
    result.append({'path':os.path.relpath(full,root).replace(os.sep,'/'),'size':stat.st_size,'checksum':h.hexdigest(),'modified':stat.st_mtime_ns})
print(json.dumps(result,separators=(',',':')))`
	output, err := a.executor.Run(ctx, "python3", []string{"-c", script, root}, nil)
	if err != nil {
		return nil, err
	}
	var files []remoteArtifactFile
	if err := json.Unmarshal(output, &files); err != nil {
		return nil, fmt.Errorf("decode remote artifact snapshot: %w", err)
	}
	return files, nil
}

func (a *Adapter) collectArtifacts(ctx context.Context, handle domain.ActivityHandle, exitCode int) (*domain.ArtifactManifest, error) {
	root, _ := handle.Metadata["artifactObservationRoot"].(string)
	encoded, _ := handle.Metadata["artifactObservationBefore"].(string)
	if root == "" {
		return nil, fmt.Errorf("remote workspace metadata is missing")
	}
	var before []remoteArtifactFile
	if encoded != "" {
		if err := json.Unmarshal([]byte(encoded), &before); err != nil {
			return nil, fmt.Errorf("decode initial remote snapshot: %w", err)
		}
	}
	after, err := a.snapshot(ctx, root)
	if err != nil {
		return nil, err
	}
	return remoteArtifactManifest(handle, root, exitCode, before, after), nil
}

func remoteArtifactManifest(handle domain.ActivityHandle, root string, exitCode int, before, after []remoteArtifactFile) *domain.ArtifactManifest {
	initial, final := indexRemoteFiles(before), indexRemoteFiles(after)
	paths := make([]string, 0, len(initial)+len(final))
	seen := map[string]bool{}
	for path := range initial {
		seen[path] = true
		paths = append(paths, path)
	}
	for path := range final {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	manifest := &domain.ArtifactManifest{SchemaVersion: 1, RunID: handle.RunID, ActivityID: handle.ActivityID,
		Attempt: 1, Runtime: handle.RuntimeID, Root: root, StartedAt: handle.StartedAt, FinishedAt: handle.FinishedAt,
		ExitCode: exitCode, Files: []domain.ArtifactObservation{}}
	manifest.InitialSnapshot = remoteSnapshotEntries(before)
	manifest.FinalSnapshot = remoteSnapshotEntries(after)
	manifest.Summary.InitialFiles, manifest.Summary.FinalFiles = len(initial), len(final)
	for _, path := range paths {
		old, hadOld := initial[path]
		current, hasCurrent := final[path]
		change := domain.ArtifactCreated
		switch {
		case !hadOld:
			manifest.Summary.CreatedFiles++
		case !hasCurrent:
			change = domain.ArtifactDeleted
			manifest.Summary.DeletedFiles++
			current = old
		case old.Size == current.Size && old.Checksum == current.Checksum:
			continue
		default:
			change = domain.ArtifactModified
			manifest.Summary.ModifiedFiles++
		}
		manifest.Files = append(manifest.Files, domain.ArtifactObservation{Path: path, Change: change,
			SizeBytes: current.Size, Checksum: "sha256:" + current.Checksum, ModifiedUnixNano: current.Modified})
		if change != domain.ArtifactDeleted {
			manifest.Summary.OutputBytes += current.Size
		}
	}
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	duration := handle.FinishedAt - handle.StartedAt
	if duration < 0 {
		duration = 0
	}
	manifest.Phases = []domain.LifecycleObservation{{Phase: "execution", Status: status, StartedAt: handle.StartedAt,
		FinishedAt: handle.FinishedAt, DurationSeconds: duration}}
	return manifest
}

func remoteSnapshotEntries(files []remoteArtifactFile) []domain.ArtifactSnapshotEntry {
	entries := make([]domain.ArtifactSnapshotEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, domain.ArtifactSnapshotEntry{Path: file.Path, SizeBytes: file.Size,
			Checksum: "sha256:" + file.Checksum, ModifiedUnixNano: file.Modified})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func indexRemoteFiles(files []remoteArtifactFile) map[string]remoteArtifactFile {
	result := make(map[string]remoteArtifactFile, len(files))
	for _, file := range files {
		result[file.Path] = file
	}
	return result
}

func (a *Adapter) Stop(ctx context.Context, handle domain.ActivityHandle) error {
	_, err := a.executor.Run(ctx, "docker", []string{"rm", "--force", handle.ExternalID}, nil)
	return err
}

func (a *Adapter) Release(ctx context.Context, handle domain.ActivityHandle) error {
	_, err := a.executor.Run(ctx, "docker", []string{"rm", "--force", handle.ExternalID}, nil)
	return err
}

func directDocker(c domain.EnvironmentConnection) bool {
	value, _ := c.Configuration["adapter"].(string)
	return value == "ssh-docker"
}
func intConfig(c map[string]any, key string) int {
	switch v := c[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 22
	}
}
func credentialFile(ref string) string { return strings.TrimPrefix(ref, "file:") }
func safeName(value string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, value)
}

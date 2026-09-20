package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type ActivityWorkspaceManager struct {
	Resolver interface {
		ResolveTransferEndpoint(context.Context, domain.TransferLocation) (domain.TransferEndpoint, error)
	}
	Kubernetes interface {
		RunWorkspaceScript(context.Context, domain.TransferEndpoint, string, io.Reader) ([]byte, error)
	}
}

func (m ActivityWorkspaceManager) Ensure(ctx context.Context, workspace domain.ActivityWorkspace) error {
	endpoint, path, err := m.endpoint(ctx, workspace)
	if err != nil {
		return err
	}
	marker := workspaceMarkerFor(workspace, path)
	if strings.HasPrefix(endpoint.URI, "file://") {
		if err := validateWorkspaceIdentity(workspace, path); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o750); err != nil {
			return err
		}
		return os.WriteFile(marker, markerContents(workspace), 0o600)
	}
	if isKubernetesEndpoint(endpoint) {
		if m.Kubernetes == nil {
			return fmt.Errorf("Kubernetes workspace lifecycle runner is unavailable")
		}
		command := "set -eu; mkdir -p -- " + shell(path) + "; printf '%s\\n%s\\n%s\\n' " +
			shell(workspace.RunID) + " " + shell(workspace.ActivityID) + " " + shell(workspace.ID) +
			" > " + shell(marker) + "; chmod 600 -- " + shell(marker)
		_, err := m.Kubernetes.RunWorkspaceScript(ctx, endpoint, command, nil)
		return err
	}
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return err
	}
	command := "set -eu; mkdir -p -- " + shell(path) + "; printf '%s\\n%s\\n%s\\n' " +
		shell(workspace.RunID) + " " + shell(workspace.ActivityID) + " " + shell(workspace.ID) +
		" > " + shell(marker) + "; chmod 600 -- " + shell(marker)
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, command), nil)
	if err != nil {
		return fmt.Errorf("create remote workspace marker: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m ActivityWorkspaceManager) Inspect(ctx context.Context, workspace domain.ActivityWorkspace) (domain.WorkspaceUsage, error) {
	endpoint, path, err := m.endpoint(ctx, workspace)
	if err != nil {
		return domain.WorkspaceUsage{}, err
	}
	if strings.HasPrefix(endpoint.URI, "file://") {
		return inspectLocalWorkspace(path)
	}
	if isKubernetesEndpoint(endpoint) {
		if m.Kubernetes == nil {
			return domain.WorkspaceUsage{}, fmt.Errorf("Kubernetes workspace lifecycle runner is unavailable")
		}
		command := "test -d " + shell(path) + " || exit 0; find " + shell(path) +
			" -type f ! -name .akoflow-owner -exec wc -c {} \\; | awk '{bytes += $1; files += 1} END {printf \"%d %d\\n\", files, bytes}'"
		output, err := m.Kubernetes.RunWorkspaceScript(ctx, endpoint, command, nil)
		return parseWorkspaceUsage(output, err)
	}
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return domain.WorkspaceUsage{}, err
	}
	command := "test -d " + shell(path) + " || exit 0; find " + shell(path) +
		" -type f ! -name .akoflow-owner -exec wc -c {} \\; | awk '{bytes += $1; files += 1} END {printf \"%d %d\\n\", files, bytes}'"
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, command), nil)
	if err != nil {
		return domain.WorkspaceUsage{}, fmt.Errorf("inspect remote workspace: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseWorkspaceUsage(output, err)
}

func parseWorkspaceUsage(output []byte, runErr error) (domain.WorkspaceUsage, error) {
	if runErr != nil {
		return domain.WorkspaceUsage{}, runErr
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return domain.WorkspaceUsage{}, nil
	}
	if len(fields) != 2 {
		return domain.WorkspaceUsage{}, fmt.Errorf("unexpected remote workspace usage %q", strings.TrimSpace(string(output)))
	}
	files, filesErr := strconv.ParseInt(fields[0], 10, 64)
	bytes, bytesErr := strconv.ParseInt(fields[1], 10, 64)
	if filesErr != nil || bytesErr != nil {
		return domain.WorkspaceUsage{}, fmt.Errorf("parse remote workspace usage %q", strings.TrimSpace(string(output)))
	}
	return domain.WorkspaceUsage{FileCount: files, SizeBytes: bytes}, nil
}

func (m ActivityWorkspaceManager) PruneInputs(ctx context.Context, workspace domain.ActivityWorkspace) (domain.WorkspaceReleaseResult, error) {
	endpoint, path, err := m.endpoint(ctx, workspace)
	if err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	if strings.HasPrefix(endpoint.URI, "file://") {
		return pruneLocalWorkspaceInputs(workspace, path)
	}
	payload, err := json.Marshal(workspace.Manifest.Inputs)
	if err != nil {
		return domain.WorkspaceReleaseResult{}, fmt.Errorf("encode workspace inputs: %w", err)
	}
	command := "python3 -c " + shell(remotePruneInputsScript) + " " + shell(path) + " " +
		shell(workspaceMarkerFor(workspace, path)) + " " + shell(strings.TrimSpace(string(markerContents(workspace))))
	if isKubernetesEndpoint(endpoint) {
		if m.Kubernetes == nil {
			return domain.WorkspaceReleaseResult{}, fmt.Errorf("Kubernetes workspace lifecycle runner is unavailable")
		}
		output, runErr := m.Kubernetes.RunWorkspaceScript(ctx, endpoint,
			kubernetesPruneCommand(path, workspace), strings.NewReader(kubernetesPrunePayload(workspace.Manifest.Inputs)))
		return decodeWorkspacePruneResult(output, runErr)
	}
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, command), bytes.NewReader(payload))
	if err != nil {
		return domain.WorkspaceReleaseResult{}, fmt.Errorf("prune remote workspace inputs: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return decodeWorkspacePruneResult(output, nil)
}

func decodeWorkspacePruneResult(output []byte, runErr error) (domain.WorkspaceReleaseResult, error) {
	if runErr != nil {
		return domain.WorkspaceReleaseResult{}, runErr
	}
	var result domain.WorkspaceReleaseResult
	if err := json.Unmarshal(output, &result); err != nil {
		return domain.WorkspaceReleaseResult{}, fmt.Errorf("decode workspace prune result %q: %w", strings.TrimSpace(string(output)), err)
	}
	return result, nil
}

func (m ActivityWorkspaceManager) Release(ctx context.Context, workspace domain.ActivityWorkspace) (domain.WorkspaceReleaseResult, error) {
	endpoint, path, err := m.endpoint(ctx, workspace)
	if err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	usage, err := m.Inspect(ctx, workspace)
	if err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	if strings.HasPrefix(endpoint.URI, "file://") {
		if err := releaseLocalWorkspace(workspace, path); err != nil {
			return domain.WorkspaceReleaseResult{}, err
		}
		return domain.WorkspaceReleaseResult{ReclaimedBytes: usage.SizeBytes}, nil
	}
	marker := workspaceMarkerFor(workspace, path)
	expected := strings.TrimSpace(string(markerContents(workspace)))
	command := "set -eu; if test ! -e " + shell(path) + "; then rm -f -- " + shell(marker) + "; exit 0; fi; " +
		"test ! -L " + shell(path) + "; canonical=$(realpath -- " + shell(path) + "); " +
		"test \"$canonical\" = " + shell(filepath.Clean(path)) + "; test -f " + shell(marker) + "; " +
		"test \"$(cat -- " + shell(marker) + ")\" = " + shell(expected) + "; " +
		"rm -rf --one-file-system -- \"$canonical\"; rm -f -- " + shell(marker)
	if isKubernetesEndpoint(endpoint) {
		if m.Kubernetes == nil {
			return domain.WorkspaceReleaseResult{}, fmt.Errorf("Kubernetes workspace lifecycle runner is unavailable")
		}
		kubernetesCommand := "set -eu; test -f " + shell(marker) + "; test \"$(cat -- " + shell(marker) + ")\" = " + shell(expected) + "; " +
			"for entry in " + shell(path) + "/* " + shell(path) + "/.[!.]* " + shell(path) + "/..?*; do " +
			"[ -e \"$entry\" ] || [ -L \"$entry\" ] || continue; rm -rf -- \"$entry\"; done"
		if _, err := m.Kubernetes.RunWorkspaceScript(ctx, endpoint, kubernetesCommand, nil); err != nil {
			return domain.WorkspaceReleaseResult{}, err
		}
		return domain.WorkspaceReleaseResult{ReclaimedBytes: usage.SizeBytes}, nil
	}
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, command), nil)
	if err != nil {
		return domain.WorkspaceReleaseResult{}, fmt.Errorf("release remote workspace: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return domain.WorkspaceReleaseResult{ReclaimedBytes: usage.SizeBytes}, nil
}

func (m ActivityWorkspaceManager) endpoint(ctx context.Context, workspace domain.ActivityWorkspace) (domain.TransferEndpoint, string, error) {
	if err := validateWorkspaceIdentity(workspace, workspace.ExecutionPath); err != nil {
		return domain.TransferEndpoint{}, "", err
	}
	u := &url.URL{Scheme: "file", Path: workspace.ExecutionPath}
	if parsed, parseErr := url.Parse(workspace.URI); parseErr == nil && parsed.Scheme == "kubernetes" {
		u = parsed
	}
	if workspace.ConnectionID != "" {
		query := u.Query()
		query.Set("connectionId", workspace.ConnectionID)
		u.RawQuery = query.Encode()
	}
	location := domain.TransferLocation{
		URI: u.String(), ResourceID: workspace.ResourceID, EnvironmentID: workspace.EnvironmentID,
		RuntimeID: workspace.RuntimeID, ConnectionID: workspace.ConnectionID,
	}
	if workspace.ConnectionID == "" {
		return domain.TransferEndpoint{URI: location.URI}, workspace.ExecutionPath, nil
	}
	if m.Resolver == nil {
		return domain.TransferEndpoint{}, "", fmt.Errorf("workspace endpoint resolver is unavailable")
	}
	endpoint, err := m.Resolver.ResolveTransferEndpoint(ctx, location)
	return endpoint, workspace.ExecutionPath, err
}

func validateWorkspaceIdentity(workspace domain.ActivityWorkspace, path string) error {
	if workspace.RunID == "" || workspace.ActivityID == "" || workspace.ID == "" {
		return fmt.Errorf("workspace identity is incomplete")
	}
	if strings.ContainsAny(workspace.RunID+workspace.ActivityID, "/\\") {
		return fmt.Errorf("workspace identity contains a path separator")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || clean == string(filepath.Separator) {
		return fmt.Errorf("workspace path %q is not a safe absolute path", path)
	}
	if workspace.Driver == "kubernetes-pvc" {
		u, err := url.Parse(workspace.URI)
		if err != nil || u.Scheme != "kubernetes" || u.Query().Get("runId") != workspace.RunID || u.Query().Get("activityId") != workspace.ActivityID {
			return fmt.Errorf("Kubernetes workspace identity does not match its endpoint")
		}
		return nil
	}
	want := filepath.Join(workspace.RunID, workspace.ActivityID)
	if !strings.HasSuffix(clean, string(filepath.Separator)+want) {
		return fmt.Errorf("workspace path %q does not end in run/activity identity", path)
	}
	return nil
}

func isKubernetesEndpoint(endpoint domain.TransferEndpoint) bool {
	u, err := url.Parse(endpoint.URI)
	return err == nil && u.Scheme == "kubernetes"
}

func workspaceMarker(path string) string { return filepath.Clean(path) + ".akoflow-owner" }

func workspaceMarkerFor(workspace domain.ActivityWorkspace, path string) string {
	if workspace.Driver == "kubernetes-pvc" {
		return filepath.Join(filepath.Clean(path), ".akoflow-owner")
	}
	return workspaceMarker(path)
}

func markerContents(workspace domain.ActivityWorkspace) []byte {
	return []byte(workspace.RunID + "\n" + workspace.ActivityID + "\n" + workspace.ID + "\n")
}

func inspectLocalWorkspace(path string) (domain.WorkspaceUsage, error) {
	usage := domain.WorkspaceUsage{}
	err := filepath.WalkDir(path, func(name string, entry os.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() && entry.Name() != ".akoflow-owner" {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			usage.FileCount++
			usage.SizeBytes += info.Size()
		}
		return nil
	})
	return usage, err
}

func releaseLocalWorkspace(workspace domain.ActivityWorkspace, path string) error {
	if err := validateWorkspaceIdentity(workspace, path); err != nil {
		return err
	}
	marker := workspaceMarkerFor(workspace, path)
	actual, err := os.ReadFile(marker)
	if os.IsNotExist(err) && os.IsNotExist(pathError(path)) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read workspace marker: %w", err)
	}
	if string(actual) != string(markerContents(workspace)) {
		return fmt.Errorf("workspace marker does not match persisted ownership")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return os.Remove(marker)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("workspace path is a symlink")
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.Remove(marker)
}

func pruneLocalWorkspaceInputs(workspace domain.ActivityWorkspace, path string) (domain.WorkspaceReleaseResult, error) {
	if err := validateWorkspaceIdentity(workspace, path); err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	actual, err := os.ReadFile(workspaceMarkerFor(workspace, path))
	if err != nil {
		return domain.WorkspaceReleaseResult{}, fmt.Errorf("read workspace marker: %w", err)
	}
	if string(actual) != string(markerContents(workspace)) {
		return domain.WorkspaceReleaseResult{}, fmt.Errorf("workspace marker does not match persisted ownership")
	}
	result := domain.WorkspaceReleaseResult{}
	for _, entry := range workspace.Manifest.Inputs {
		file, err := safeWorkspaceEntry(path, entry.Path)
		if err != nil {
			return result, err
		}
		info, err := os.Lstat(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		if !info.Mode().IsRegular() || info.Size() != entry.SizeBytes {
			result.PreservedFiles++
			continue
		}
		digest, err := checksumFile(file)
		if err != nil {
			return result, err
		}
		if "sha256:"+digest != entry.Digest {
			result.PreservedFiles++
			continue
		}
		if err := os.Remove(file); err != nil {
			return result, err
		}
		result.RemovedFiles++
		result.ReclaimedBytes += info.Size()
	}
	removeEmptyWorkspaceDirectories(path)
	return result, nil
}

func safeWorkspaceEntry(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("workspace entry %q is not relative", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace entry %q escapes its root", relative)
	}
	return filepath.Join(root, clean), nil
}

func checksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := file.WriteTo(hash); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func removeEmptyWorkspaceDirectories(root string) {
	var directories []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && entry.IsDir() && path != root {
			directories = append(directories, path)
		}
		return nil
	})
	for index := len(directories) - 1; index >= 0; index-- {
		_ = os.Remove(directories[index])
	}
}

const remotePruneInputsScript = `import hashlib,json,os,sys
root,marker,expected=sys.argv[1:4]
with open(marker,'r',encoding='utf-8') as stream:
  if stream.read().strip()!=expected: raise SystemExit('workspace marker does not match persisted ownership')
root_real=os.path.realpath(root)
removed=preserved=reclaimed=0
for entry in json.load(sys.stdin):
  relative=entry.get('path','')
  if not relative or os.path.isabs(relative): raise SystemExit('unsafe workspace input: '+relative)
  target=os.path.normpath(os.path.join(root_real,relative))
  if os.path.commonpath([root_real,target])!=root_real: raise SystemExit('workspace input escapes root: '+relative)
  try: stat=os.lstat(target)
  except FileNotFoundError: continue
  if not os.path.isfile(target) or os.path.islink(target) or stat.st_size!=entry.get('sizeBytes'): preserved+=1; continue
  digest=hashlib.sha256()
  with open(target,'rb') as stream:
    for chunk in iter(lambda: stream.read(1024*1024),b''): digest.update(chunk)
  if 'sha256:'+digest.hexdigest()!=entry.get('digest'): preserved+=1; continue
  os.unlink(target); removed+=1; reclaimed+=stat.st_size
for base,dirs,files in os.walk(root_real,topdown=False):
  if base!=root_real:
    try: os.rmdir(base)
    except OSError: pass
print(json.dumps({'reclaimedBytes':reclaimed,'removedFiles':removed,'preservedFiles':preserved},separators=(',',':')))`

func kubernetesPrunePayload(entries []domain.WorkspaceEntry) string {
	var payload strings.Builder
	for _, entry := range entries {
		encoded := base64.StdEncoding.EncodeToString([]byte(entry.Path))
		fmt.Fprintf(&payload, "%s|%d|%s\n", encoded, entry.SizeBytes, strings.TrimPrefix(entry.Digest, "sha256:"))
	}
	return payload.String()
}

func kubernetesPruneCommand(path string, workspace domain.ActivityWorkspace) string {
	marker := workspaceMarkerFor(workspace, path)
	expected := strings.TrimSpace(string(markerContents(workspace)))
	return "set -eu; test -f " + shell(marker) + "; test \"$(cat -- " + shell(marker) + ")\" = " + shell(expected) + `; ` +
		`removed=0; preserved=0; reclaimed=0; while IFS='|' read -r encoded size digest; do ` +
		`relative=$(printf '%s' "$encoded" | base64 -d); case "$relative" in ''|/*|..|../*|*/../*) echo "unsafe workspace input: $relative" >&2; exit 65;; esac; ` +
		"target=" + shell(path) + `/"$relative"; [ -e "$target" ] || continue; ` +
		`if [ -L "$target" ] || [ ! -f "$target" ] || [ "$(wc -c < "$target")" -ne "$size" ]; then preserved=$((preserved+1)); continue; fi; ` +
		`actual=$(sha256sum "$target" | awk '{print $1}'); if [ "$actual" != "$digest" ]; then preserved=$((preserved+1)); continue; fi; ` +
		`rm -f -- "$target"; removed=$((removed+1)); reclaimed=$((reclaimed+size)); done; ` +
		`printf '{"reclaimedBytes":%s,"removedFiles":%s,"preservedFiles":%s}\n' "$reclaimed" "$removed" "$preserved"`
}

func pathError(path string) error {
	_, err := os.Lstat(path)
	return err
}

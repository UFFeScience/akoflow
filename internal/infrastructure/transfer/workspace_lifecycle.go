package transfer

import (
	"context"
	"fmt"
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
}

func (m ActivityWorkspaceManager) Ensure(ctx context.Context, workspace domain.ActivityWorkspace) error {
	endpoint, path, err := m.endpoint(ctx, workspace)
	if err != nil {
		return err
	}
	if strings.HasPrefix(endpoint.URI, "file://") {
		if err := validateWorkspaceIdentity(workspace, path); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o750); err != nil {
			return err
		}
		return os.WriteFile(workspaceMarker(path), markerContents(workspace), 0o600)
	}
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return err
	}
	command := "set -eu; mkdir -p -- " + shell(path) + "; printf '%s\\n%s\\n%s\\n' " +
		shell(workspace.RunID) + " " + shell(workspace.ActivityID) + " " + shell(workspace.ID) +
		" > " + shell(workspaceMarker(path)) + "; chmod 600 -- " + shell(workspaceMarker(path))
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
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return domain.WorkspaceUsage{}, err
	}
	command := "test -d " + shell(path) + " || exit 0; find " + shell(path) +
		" -type f -printf '%s\\n' | awk '{bytes += $1; files += 1} END {printf \"%d %d\\n\", files, bytes}'"
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, command), nil)
	if err != nil {
		return domain.WorkspaceUsage{}, fmt.Errorf("inspect remote workspace: %w: %s", err, strings.TrimSpace(string(output)))
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
	host, _, err := sshTarget(endpoint, "")
	if err != nil {
		return domain.WorkspaceReleaseResult{}, err
	}
	marker := workspaceMarker(path)
	expected := strings.TrimSpace(string(markerContents(workspace)))
	command := "set -eu; if test ! -e " + shell(path) + "; then rm -f -- " + shell(marker) + "; exit 0; fi; " +
		"test ! -L " + shell(path) + "; canonical=$(realpath -- " + shell(path) + "); " +
		"test \"$canonical\" = " + shell(filepath.Clean(path)) + "; test -f " + shell(marker) + "; " +
		"test \"$(cat -- " + shell(marker) + ")\" = " + shell(expected) + "; " +
		"rm -rf --one-file-system -- \"$canonical\"; rm -f -- " + shell(marker)
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
	want := filepath.Join(workspace.RunID, workspace.ActivityID)
	if !strings.HasSuffix(clean, string(filepath.Separator)+want) {
		return fmt.Errorf("workspace path %q does not end in run/activity identity", path)
	}
	return nil
}

func workspaceMarker(path string) string { return filepath.Clean(path) + ".akoflow-owner" }

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
		if entry.Type().IsRegular() {
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
	marker := workspaceMarker(path)
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

func pathError(path string) error {
	_, err := os.Lstat(path)
	return err
}

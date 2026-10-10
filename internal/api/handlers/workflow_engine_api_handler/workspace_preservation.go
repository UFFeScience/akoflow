package workflow_engine_api_handler

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/google/uuid"
)

type preservedWorkspaceStore interface {
	ReleasePreservedWorkspaces(context.Context, string, string, []string, string) error
}

// PreserveCancelledCloudWorkspaces copies an exact-generation archive to the
// relay before releasing consumer leases. It never removes source files or VMs.
func (h *Handler) PreserveCancelledCloudWorkspaces(w http.ResponseWriter, r *http.Request) {
	store, ok := h.executions.(preservedWorkspaceStore)
	query, queryOK := h.executions.(WorkspaceQuery)
	if h.readOnly || !ok || !queryOK || h.storage == nil {
		writeError(w, http.StatusForbidden, fmt.Errorf("workspace preservation is unavailable"))
		return
	}
	var input struct {
		DownloadID string `json:"downloadId"`
		InstanceID string `json:"instanceId"`
		SHA256     string `json:"sha256"`
	}
	if !decode(w, r, &input) {
		return
	}
	runID := r.PathValue("runId")
	if path.Base(runID) != runID || runID == "." || !strings.HasPrefix(input.InstanceID, "cloud-instance-") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("explicit run and instance identities required"))
		return
	}
	want := strings.TrimPrefix(input.SHA256, "sha256:")
	decoded, err := hex.DecodeString(want)
	if err != nil || len(decoded) != sha256.Size {
		writeError(w, http.StatusBadRequest, fmt.Errorf("SHA-256 required"))
		return
	}
	run, err := h.executions.FindRun(r.Context(), runID)
	if err != nil || run == nil || run.Status != domain.ExecutionRunCancelled {
		writeError(w, http.StatusConflict, fmt.Errorf("only cancelled runs can release preserved workspaces"))
		return
	}
	handles, err := h.executions.ListHandles(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, handle := range handles {
		if handle.Status != domain.HandleCompleted && handle.Status != domain.HandleFailed && handle.Status != domain.HandleStopped {
			writeError(w, http.StatusConflict, fmt.Errorf("execution handles are still active"))
			return
		}
	}
	download, err := h.storage.Download(r.Context(), input.DownloadID)
	archivePath := "runs/" + runID + ".tar.gz"
	if err != nil || download == nil || string(download.Status) != "ready" || download.Strategy != "archive" || download.StorageID != "cloud-workspace:"+input.InstanceID || download.Path != archivePath {
		writeError(w, http.StatusConflict, fmt.Errorf("archive does not match exact run and cloud generation"))
		return
	}
	remoteDigest, err := h.storage.Checksum(r.Context(), download.StorageID, download.Path)
	if err != nil || strings.TrimPrefix(remoteDigest, "sha256:") != want {
		writeError(w, http.StatusConflict, fmt.Errorf("source archive checksum differs"))
		return
	}
	workspaces, err := query.ListWorkspaces(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	selected := []domain.ActivityWorkspace{}
	for _, workspace := range workspaces {
		location, parseErr := url.Parse(workspace.URI)
		if parseErr == nil && location.Query().Get("cloudInstanceId") == input.InstanceID && workspace.State != domain.WorkspaceReleased {
			if workspace.Pinned || path.Base(workspace.ActivityID) != workspace.ActivityID {
				writeError(w, http.StatusConflict, fmt.Errorf("workspace cannot be released"))
				return
			}
			selected = append(selected, workspace)
		}
	}
	if len(selected) == 0 {
		writeError(w, http.StatusConflict, fmt.Errorf("no retained workspaces match this generation"))
		return
	}
	reader, _, err := h.storage.OpenDownload(r.Context(), download.ID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	defer reader.Close()
	relayPath, err := preserveArchive(reader, "storage/preserved-workspaces", want, runID, selected)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	ids := make([]string, 0, len(selected))
	for _, workspace := range selected {
		ids = append(ids, workspace.ID)
	}
	if err := store.ReleasePreservedWorkspaces(r.Context(), runID, input.InstanceID, ids, relayPath+" sha256:"+want); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "preserved", "archive": relayPath, "sha256": want, "workspaceIds": ids, "instanceId": input.InstanceID})
}

func preserveArchive(reader io.Reader, directory, digest, runID string, workspaces []domain.ActivityWorkspace) (string, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	temporary := ".pending-" + uuid.NewString()
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	defer root.Remove(temporary)
	hash := sha256.New()
	const maxArchive = int64(50 << 30)
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, maxArchive+1))
	if err != nil || size == 0 || size > maxArchive {
		return "", fmt.Errorf("archive copy is empty, oversized or failed: %v", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return "", fmt.Errorf("relay archive checksum differs")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if err := validatePreservedArchive(file, runID, workspaces); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	name := digest + ".tar.gz"
	if err := root.Rename(temporary, name); err != nil {
		return "", err
	}
	dir, err := root.Open(".")
	if err != nil {
		return "", err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return "", err
	}
	return path.Join(directory, name), nil
}

func validatePreservedArchive(reader io.Reader, runID string, workspaces []domain.ActivityWorkspace) error {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gz.Close()
	tarReader := tar.NewReader(gz)
	prefix := "runs/" + runID
	seen := map[string]bool{}
	expected := map[string]domain.WorkspaceEntry{}
	for _, workspace := range workspaces {
		for _, entry := range workspace.Manifest.Final {
			expected[prefix+"/"+workspace.ActivityID+"/"+entry.Path] = entry
		}
	}
	files := 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(header.Name, "/")
		if path.Clean(name) != name || (name != prefix && !strings.HasPrefix(name, prefix+"/")) || (header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg) {
			return fmt.Errorf("archive contains unsafe or unrelated entry")
		}
		seen[name] = true
		if header.Typeflag == tar.TypeReg {
			files++
		}
		if entry, ok := expected[name]; ok && header.Typeflag == tar.TypeReg {
			digest := sha256.New()
			size, err := io.Copy(digest, tarReader)
			if err != nil {
				return err
			}
			if size != entry.SizeBytes || strings.TrimPrefix(entry.Digest, "sha256:") != hex.EncodeToString(digest.Sum(nil)) {
				return fmt.Errorf("archive differs from sealed workspace manifest")
			}
		} else if _, err := io.Copy(io.Discard, tarReader); err != nil {
			return err
		}
	}
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return err
	} // validate gzip footer
	if files == 0 {
		return fmt.Errorf("archive contains no files")
	}
	for _, workspace := range workspaces {
		if !seen[prefix+"/"+workspace.ActivityID] {
			return fmt.Errorf("archive omits retained workspace %s", workspace.ActivityID)
		}
		for _, entry := range workspace.Manifest.Final {
			if !seen[prefix+"/"+workspace.ActivityID+"/"+entry.Path] {
				return fmt.Errorf("archive omits workspace manifest entry")
			}
		}
	}
	return nil
}

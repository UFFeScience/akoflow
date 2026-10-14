package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// A seal without a complete content manifest is not an immutable snapshot
// identity. Such legacy workspaces continue to transfer without cache reuse.
func sealedWorkspaceSnapshot(workspace *domain.ActivityWorkspace, instanceID string) (string, []domain.WorkspaceEntry) {
	if workspace == nil || instanceID == "" || workspace.SealedAt == nil ||
		(workspace.State != domain.WorkspaceSealed && workspace.State != domain.WorkspaceReleasable) ||
		len(workspace.Manifest.Final) == 0 || int64(len(workspace.Manifest.Final)) != workspace.FileCount {
		return "", nil
	}
	u, err := url.Parse(workspace.URI)
	if err != nil || u.Query().Get("cloudInstanceId") != instanceID {
		return "", nil
	}
	entries := append([]domain.WorkspaceEntry(nil), workspace.Manifest.Final...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	for i, entry := range entries {
		if entry.Path == "" || entry.Path == "." || path.IsAbs(entry.Path) || path.Clean(entry.Path) != entry.Path ||
			strings.HasPrefix(entry.Path, "../") || strings.ContainsRune(entry.Path, 0) || entry.SizeBytes < 0 ||
			(i > 0 && entries[i-1].Path == entry.Path) || !strings.HasPrefix(entry.Digest, "sha256:") {
			return "", nil
		}
		digest, err := hex.DecodeString(strings.TrimPrefix(entry.Digest, "sha256:"))
		if err != nil || len(digest) != sha256.Size {
			return "", nil
		}
	}
	encoded, err := json.Marshal(struct {
		WorkspaceID, URI, InstanceID, Seal string
		Entries                            []domain.WorkspaceEntry
	}{workspace.ID, workspace.URI, instanceID, workspace.SealedAt.UTC().Format("2006-01-02T15:04:05.999999999Z"), entries})
	if err != nil {
		return "", nil
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), entries
}

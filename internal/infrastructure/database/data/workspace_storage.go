package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func registerWorkspaceStorage(ctx context.Context, tx *sql.Tx, handle domain.ActivityHandle, environmentVersionID string, storageType domain.StorageType) (string, error) {
	if configured, _ := handle.Metadata["artifactStorageResourceId"].(string); configured != "" {
		return configured, nil
	}
	root := path.Clean(handle.Artifacts.Root)
	if !path.IsAbs(root) || root == "/" {
		return "", fmt.Errorf("workspace storage requires a bounded absolute root")
	}
	metadata := map[string]any{"managedBy": "akoflow"}
	var uri, connectionID string
	err := tx.QueryRowContext(ctx, `SELECT uri,connection_id FROM activity_workspaces
 WHERE execution_run_id=? AND activity_id=? AND resource_id=?`, handle.RunID, handle.ActivityID, handle.ResourceID).Scan(&uri, &connectionID)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("resolve workspace storage: %w", err)
	}
	roots := []domain.StorageBrowseRoot{{Path: root, Label: "Activity workspace"}}
	if workspace, parseErr := url.Parse(uri); parseErr == nil && (workspace.Scheme == "ssh" || workspace.Query().Get("cloudInstanceId") != "") {
		storageType = domain.StorageSSH
		if instanceID := workspace.Query().Get("cloudInstanceId"); instanceID != "" {
			metadata["cloudInstanceId"] = instanceID
		} else if connectionID != "" {
			metadata["connectionId"] = connectionID
		} else {
			roots = nil
			metadata["unavailableReason"] = "Remote workspace has no connection identity"
		}
	} else if uri != "" && (parseErr != nil || workspace.Scheme != "file") {
		roots = nil
		metadata["unavailableReason"] = "Workspace requires a runtime-specific storage connection"
	} else if target, _ := handle.Metadata["executionTarget"].(string); strings.HasPrefix(target, "remote-") {
		roots = nil
		metadata["unavailableReason"] = "Remote workspace connection is not recorded"
	}
	configuration, err := json.Marshal(map[string]any{"browseRoots": roots})
	if err != nil {
		return "", err
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	storageID := stableID("workspace-storage", environmentVersionID, handle.ResourceID, string(storageType), root, string(encodedMetadata))
	_, err = tx.ExecContext(ctx, `INSERT INTO storage_resources (
 id,environment_version_id,name,type,endpoint,shared,configuration,metadata
 ) VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, storageID, environmentVersionID,
		"workspace-"+handle.ActivityID+"-"+storageID, storageType, root,
		storageType == domain.StoragePVC || storageType == domain.StorageNFS, string(configuration), string(encodedMetadata))
	if err != nil {
		return "", fmt.Errorf("register workspace storage: %w", err)
	}
	return storageID, nil
}

package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// ArtifactSnapshot is a portable filesystem observation taken around an
// activity attempt. It deliberately observes only the activity workspace.
type ArtifactSnapshot struct {
	Root  string
	Files map[string]artifactFile
}

type artifactFile struct {
	size     int64
	checksum string
	modified int64
}

// PrepareArtifactRoot returns the declared workspace or allocates an isolated
// one for activities that did not declare a working directory.
func PrepareArtifactRoot(activity domain.Activity, runID string) (string, error) {
	if activity.Metadata != nil {
		if root, ok := activity.Metadata["artifactObservationRoot"].(string); ok && root != "" {
			return root, os.MkdirAll(root, 0o700)
		}
	}
	if activity.Command.WorkingDirectory != "" {
		return activity.Command.WorkingDirectory, os.MkdirAll(activity.Command.WorkingDirectory, 0o700)
	}
	return os.MkdirTemp("", "akoflow-"+safePathToken(runID)+"-"+safePathToken(activity.ID)+"-")
}

func SnapshotArtifacts(root string) (ArtifactSnapshot, error) {
	snapshot := ArtifactSnapshot{Root: root, Files: make(map[string]artifactFile)}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		checksum, err := fileChecksum(path)
		if err != nil {
			return err
		}
		snapshot.Files[filepath.ToSlash(relative)] = artifactFile{size: info.Size(), checksum: checksum, modified: info.ModTime().UnixNano()}
		return nil
	})
	return snapshot, err
}

func ArtifactManifestFor(runID, activityID, runtime string, startedAt, finishedAt float64, exitCode int, before, after ArtifactSnapshot) *domain.ArtifactManifest {
	manifest := &domain.ArtifactManifest{SchemaVersion: 1, RunID: runID, ActivityID: activityID,
		Attempt: 1, Runtime: runtime, Root: after.Root, StartedAt: startedAt, FinishedAt: finishedAt,
		ExitCode: exitCode, Files: make([]domain.ArtifactObservation, 0)}
	paths := make([]string, 0, len(before.Files)+len(after.Files))
	seen := make(map[string]bool)
	for path := range before.Files {
		seen[path] = true
		paths = append(paths, path)
	}
	for path := range after.Files {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		previous, hadPrevious := before.Files[path]
		current, hasCurrent := after.Files[path]
		change := domain.ArtifactCreated
		switch {
		case !hadPrevious:
			manifest.Summary.CreatedFiles++
		case !hasCurrent:
			change = domain.ArtifactDeleted
			manifest.Summary.DeletedFiles++
		case previous.size == current.size && previous.checksum == current.checksum:
			continue
		default:
			change = domain.ArtifactModified
			manifest.Summary.ModifiedFiles++
		}
		file := current
		if !hasCurrent {
			file = previous
		}
		manifest.Files = append(manifest.Files, domain.ArtifactObservation{Path: path, Change: change,
			SizeBytes: file.size, Checksum: "sha256:" + file.checksum, ModifiedUnixNano: file.modified})
		if change != domain.ArtifactDeleted {
			manifest.Summary.OutputBytes += file.size
		}
	}
	manifest.Summary.InitialFiles = len(before.Files)
	manifest.Summary.FinalFiles = len(after.Files)
	manifest.Phases = []domain.LifecycleObservation{{Phase: "execution", Status: artifactPhase(exitCode), StartedAt: startedAt, FinishedAt: finishedAt, DurationSeconds: maxDuration(finishedAt - startedAt)}}
	return manifest
}

func fileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func artifactPhase(exitCode int) string {
	if exitCode == 0 {
		return "completed"
	}
	return "failed"
}

func maxDuration(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func safePathToken(value string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			return character
		}
		return '-'
	}, value)
}

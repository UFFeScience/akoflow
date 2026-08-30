package instancearchive

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	"github.com/UFFeScience/akoflow/internal/provider"
)

const (
	archiveFormat        = "akoflow-instance"
	archiveFormatVersion = 1
	maxArchiveFiles      = 100000
)

type Service struct {
	db           *sql.DB
	databasePath string
	root         string
	artifacts    string
	version      string
}

func New(db *sql.DB, databasePath, root, artifacts, version string) (*Service, error) {
	if db == nil {
		return nil, fmt.Errorf("instance archive requires a database")
	}
	if strings.TrimSpace(databasePath) == "" {
		databasePath = database.DefaultPath
	}
	absDatabase, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve active database: %w", err)
	}
	if strings.TrimSpace(root) == "" {
		root = "storage/instances"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve instance archive root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create instance archive root: %w", err)
	}
	return &Service{db: db, databasePath: absDatabase, root: absRoot, artifacts: artifacts, version: version}, nil
}

func (s *Service) List(_ context.Context) ([]domaininstance.ArchiveInstance, error) {
	items := []domaininstance.ArchiveInstance{{
		ID: "default", Name: "Default instance", Source: "local", Status: "active",
		ReadOnly: false, CredentialsSet: true, DatabasePath: s.databasePath, AkoflowVersion: s.version,
	}}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readManifest(filepath.Join(s.root, entry.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		item := manifest.Instance
		item.Contents = manifest.Contents
		item.DatabasePath = filepath.Join(s.root, entry.Name(), "database", "akoflow.sqlite")
		items = append(items, item)
	}
	sort.Slice(items[1:], func(i, j int) bool {
		left, right := items[i+1], items[j+1]
		if left.ImportedAt == nil {
			return false
		}
		if right.ImportedAt == nil {
			return true
		}
		return left.ImportedAt.After(*right.ImportedAt)
	})
	return items, nil
}

func (s *Service) Export(ctx context.Context, output io.Writer, includeArtifacts bool) error {
	temporary, err := os.MkdirTemp("", "akoflow-export-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	snapshot := filepath.Join(temporary, "akoflow.sqlite")
	if err := s.snapshot(ctx, snapshot); err != nil {
		return err
	}
	if err := redactDatabase(snapshot); err != nil {
		return err
	}
	databaseChecksum, err := fileChecksum(snapshot)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	manifest := domaininstance.ArchiveManifest{
		Format: archiveFormat, FormatVersion: archiveFormatVersion, ExportedAt: now,
		Instance: domaininstance.ArchiveInstance{
			ID: provider.NewID("instance"), Name: "AkôFlow instance", Source: "export",
			Status: "snapshot", ReadOnly: true, CredentialsSet: false,
			ExportedAt: &now, AkoflowVersion: s.version,
		},
		Security:  domaininstance.ArchiveSecurity{CredentialsIncluded: false, CredentialsRedacted: true},
		Contents:  databaseCounts(ctx, snapshot),
		Checksums: map[string]string{"database/akoflow.sqlite": databaseChecksum},
	}
	archive := zip.NewWriter(output)
	if err := addFile(archive, "database/akoflow.sqlite", snapshot); err != nil {
		_ = archive.Close()
		return err
	}
	if includeArtifacts && strings.TrimSpace(s.artifacts) != "" {
		if err := addDirectory(archive, "artifacts", s.artifacts); err != nil {
			_ = archive.Close()
			return err
		}
	}
	value, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = archive.Close()
		return err
	}
	entry, err := archive.Create("manifest.json")
	if err == nil {
		_, err = entry.Write(value)
	}
	if err != nil {
		_ = archive.Close()
		return err
	}
	return archive.Close()
}

func (s *Service) Import(_ context.Context, source io.Reader, size int64) (domaininstance.ArchiveInstance, error) {
	temporary, err := os.CreateTemp("", "akoflow-import-*.zip")
	if err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := io.Copy(temporary, io.LimitReader(source, size+1)); err != nil {
		_ = temporary.Close()
		return domaininstance.ArchiveInstance{}, err
	}
	if err := temporary.Close(); err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	info, err := os.Stat(temporaryPath)
	if err != nil || info.Size() > size {
		return domaininstance.ArchiveInstance{}, fmt.Errorf("instance archive exceeds the allowed size")
	}
	reader, err := zip.OpenReader(temporaryPath)
	if err != nil {
		return domaininstance.ArchiveInstance{}, fmt.Errorf("open instance archive: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveFiles {
		return domaininstance.ArchiveInstance{}, fmt.Errorf("instance archive contains too many files")
	}
	var uncompressed uint64
	for _, file := range reader.File {
		uncompressed += file.UncompressedSize64
		if uncompressed > 64<<30 {
			return domaininstance.ArchiveInstance{}, fmt.Errorf("instance archive expands beyond the allowed size")
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return domaininstance.ArchiveInstance{}, fmt.Errorf("instance archive contains a symbolic link")
		}
	}
	manifest, err := manifestFromArchive(reader.File)
	if err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	if manifest.Format != archiveFormat || manifest.FormatVersion != archiveFormatVersion || !manifest.Security.CredentialsRedacted {
		return domaininstance.ArchiveInstance{}, fmt.Errorf("unsupported or unsafe AkôFlow instance archive")
	}
	id := provider.NewID("instance")
	destination := filepath.Join(s.root, id)
	staging := destination + ".importing"
	defer os.RemoveAll(staging)
	if err := extractArchive(reader.File, staging); err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	databasePath := filepath.Join(staging, "database", "akoflow.sqlite")
	checksum, err := fileChecksum(databasePath)
	if err != nil || checksum != manifest.Checksums["database/akoflow.sqlite"] {
		return domaininstance.ArchiveInstance{}, fmt.Errorf("instance database checksum does not match the manifest")
	}
	importedDB, err := database.Open(databasePath)
	if err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	if err := database.Validate(context.Background(), importedDB); err != nil {
		_ = importedDB.Close()
		return domaininstance.ArchiveInstance{}, fmt.Errorf("validate imported database: %w", err)
	}
	_ = importedDB.Close()
	now := time.Now().UTC()
	manifest.Instance.ID = id
	manifest.Instance.Source = "imported"
	manifest.Instance.Status = "snapshot"
	manifest.Instance.ReadOnly = true
	manifest.Instance.CredentialsSet = false
	manifest.Instance.ImportedAt = &now
	if strings.TrimSpace(manifest.Instance.Name) == "" {
		manifest.Instance.Name = "Imported AkôFlow instance"
	}
	if err := writeManifest(filepath.Join(staging, "manifest.json"), manifest); err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	if err := os.Rename(staging, destination); err != nil {
		return domaininstance.ArchiveInstance{}, err
	}
	return manifest.Instance, nil
}

func (s *Service) snapshot(ctx context.Context, destination string) error {
	// destination is generated by the server, but quote it defensively for SQLite.
	quoted := strings.ReplaceAll(destination, "'", "''")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO '"+quoted+"'"); err != nil {
		return fmt.Errorf("create consistent database snapshot: %w", err)
	}
	return nil
}

func redactDatabase(path string) error {
	db, err := database.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`UPDATE environment_connections SET credential_ref=''`,
		`UPDATE storage_resources SET credential_reference=''`,
		`UPDATE connector_bindings SET credential_ref='', enabled=0`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("redact instance credentials: %w", err)
		}
	}
	for _, target := range []struct{ table, key, column string }{
		{"environment_connections", "id", "configuration"},
		{"environment_runtimes", "id", "configuration"},
		{"resource_runtime_bindings", "resource_id || ':' || runtime_id", "configuration"},
		{"storage_resources", "id", "configuration"},
		{"storage_runtime_bindings", "storage_resource_id || ':' || runtime_id", "configuration"},
		{"transfer_endpoints", "id", "configuration"},
	} {
		if err := redactJSONColumn(tx, target.table, target.key, target.column); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func redactJSONColumn(tx *sql.Tx, table, keyExpression, column string) error {
	query := fmt.Sprintf("SELECT %s, %s FROM %s", keyExpression, column, table)
	rows, err := tx.Query(query)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return err
	}
	defer rows.Close()
	type update struct{ key, value string }
	updates := []update{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return err
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			value = map[string]any{}
		}
		cleaned, changed := redactJSONValue(value)
		if !changed {
			continue
		}
		object, ok := cleaned.(map[string]any)
		if !ok {
			object = map[string]any{}
		}
		object["credentialStatus"] = "redacted"
		object["requiresReconfiguration"] = true
		encoded, err := json.Marshal(object)
		if err != nil {
			return err
		}
		updates = append(updates, update{key: key, value: string(encoded)})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range updates {
		statement := fmt.Sprintf("UPDATE %s SET %s=? WHERE %s=?", table, column, keyExpression)
		if _, err := tx.Exec(statement, item.value, item.key); err != nil {
			return err
		}
	}
	return nil
}

func redactJSONValue(value any) (any, bool) {
	changed := false
	switch current := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(current))
		for key, child := range current {
			if sensitiveConfigurationKey(key) {
				changed = true
				continue
			}
			redacted, childChanged := redactJSONValue(child)
			changed = changed || childChanged
			cleaned[key] = redacted
		}
		return cleaned, changed
	case []any:
		cleaned := make([]any, len(current))
		for index, child := range current {
			redacted, childChanged := redactJSONValue(child)
			changed = changed || childChanged
			cleaned[index] = redacted
		}
		return cleaned, changed
	default:
		return value, false
	}
}

func sensitiveConfigurationKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	for _, fragment := range []string{
		"token", "secret", "password", "privatekey", "clientkey", "apikey",
		"authorization", "credential", "kubeconfig", "certificatekey", "accesskey",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func databaseCounts(ctx context.Context, path string) map[string]int64 {
	result := map[string]int64{}
	db, err := database.Open(path)
	if err != nil {
		return result
	}
	defer db.Close()
	for label, table := range map[string]string{
		"workflows": "workflow_definitions", "environments": "environments",
		"plans": "schedule_plans", "runs": "execution_runs", "artifacts": "artifacts",
	} {
		var count int64
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count) == nil {
			result[label] = count
		}
	}
	return result
}

func addFile(archive *zip.Writer, name, source string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	entry, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

func addDirectory(archive *zip.Writer, prefix, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return addFile(archive, filepath.ToSlash(filepath.Join(prefix, relative)), path)
	})
}

func manifestFromArchive(files []*zip.File) (domaininstance.ArchiveManifest, error) {
	for _, file := range files {
		if file.Name != "manifest.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return domaininstance.ArchiveManifest{}, err
		}
		defer reader.Close()
		var manifest domaininstance.ArchiveManifest
		if err := json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(&manifest); err != nil {
			return manifest, fmt.Errorf("decode instance manifest: %w", err)
		}
		return manifest, nil
	}
	return domaininstance.ArchiveManifest{}, fmt.Errorf("instance archive has no manifest")
}

func extractArchive(files []*zip.File, destination string) error {
	for _, file := range files {
		clean := filepath.Clean(filepath.FromSlash(file.Name))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return fmt.Errorf("unsafe path in instance archive: %q", file.Name)
		}
		target := filepath.Join(destination, clean)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err == nil {
			_, err = io.Copy(output, input)
		}
		_ = input.Close()
		if output != nil {
			_ = output.Close()
		}
		if err != nil {
			return err
		}
	}
	return nil
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

func readManifest(path string) (domaininstance.ArchiveManifest, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return domaininstance.ArchiveManifest{}, err
	}
	var manifest domaininstance.ArchiveManifest
	err = json.Unmarshal(value, &manifest)
	return manifest, err
}

func writeManifest(path string, manifest domaininstance.ArchiveManifest) error {
	value, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, value, 0o600)
}

func ResolveDatabasePath() string {
	if value := strings.TrimSpace(os.Getenv("AKOFLOW_DATABASE_PATH")); value != "" {
		return value
	}
	return database.DefaultPath
}

func DefaultRoot() string {
	if value := strings.TrimSpace(os.Getenv("AKOFLOW_INSTANCE_ARCHIVE_ROOT")); value != "" {
		return value
	}
	return "storage/instances"
}

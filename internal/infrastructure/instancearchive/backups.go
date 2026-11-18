package instancearchive

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/provider"
)

func (s *Service) BackupStatus() (instance.BackupStatus, error) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	status, err := s.readBackupStatus()
	if err == nil {
		status.Files, _ = s.backupFiles(status.Settings.Destination)
	}
	return status, err
}

func (s *Service) readBackupStatus() (instance.BackupStatus, error) {
	status := instance.BackupStatus{Settings: instance.BackupSettings{Destination: filepath.Join(s.root, "backups"), Retain: 7}}
	value, err := os.ReadFile(filepath.Join(s.root, ".backup-status.json"))
	if errors.Is(err, os.ErrNotExist) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	return status, json.Unmarshal(value, &status)
}

func (s *Service) ConfigureBackups(settings instance.BackupSettings) (instance.BackupStatus, error) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	settings.Destination = strings.TrimSpace(settings.Destination)
	if !filepath.IsAbs(settings.Destination) || settings.IntervalHours < 0 || settings.IntervalHours > 8760 || settings.Retain < 1 || settings.Retain > 1000 {
		return instance.BackupStatus{}, errors.New("use an absolute daemon-side directory, a schedule of 0–8760 hours and a retention of 1–1000 backups")
	}
	settings.Destination = filepath.Clean(settings.Destination)
	status, err := s.readBackupStatus()
	if err != nil {
		return status, err
	}
	if settings != status.Settings {
		status.NextScheduled = time.Time{}
		if settings.IntervalHours > 0 {
			status.NextScheduled = time.Now().UTC().Add(time.Duration(settings.IntervalHours) * time.Hour)
		}
	}
	status.Settings = settings
	if err := s.writeBackupStatus(status); err != nil {
		return status, err
	}
	status.Files, _ = s.backupFiles(settings.Destination)
	return status, nil
}

func (s *Service) CreateBackup(ctx context.Context) (instance.BackupStatus, error) {
	return s.runBackup(ctx, false)
}

func (s *Service) RunBackupScheduler(ctx context.Context, report func(error)) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if _, err := s.runBackup(ctx, true); err != nil && ctx.Err() == nil {
			report(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) runBackup(ctx context.Context, scheduled bool) (instance.BackupStatus, error) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	status, err := s.readBackupStatus()
	if err != nil {
		return status, err
	}
	now := time.Now().UTC()
	if scheduled && (status.NextScheduled.IsZero() || now.Before(status.NextScheduled)) {
		return status, nil
	}
	result := &instance.BackupResult{StartedAt: now, Scheduled: scheduled}
	result.File, err = s.saveBackup(ctx, status.Settings.Destination, now)
	if err == nil {
		result.Removed, err = s.pruneBackups(status.Settings)
	}
	result.FinishedAt = time.Now().UTC()
	if err != nil {
		result.Error = err.Error()
	}
	status.Last = result
	if scheduled {
		status.LastScheduled = result
		status.NextScheduled = result.FinishedAt.Add(time.Duration(status.Settings.IntervalHours) * time.Hour)
	}
	writeErr := s.writeBackupStatus(status)
	status.Files, _ = s.backupFiles(status.Settings.Destination)
	return status, errors.Join(err, writeErr)
}

func (s *Service) saveBackup(ctx context.Context, destination string, now time.Time) (string, error) {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return "", err
	}
	name := s.backupPrefix() + now.Format("20060102T150405.000000000Z") + "-" + provider.NewID("snapshot") + ".zip"
	err := writeAtomic(destination, name, func(file *os.File) error {
		return s.Export(ctx, file, false)
	})
	if err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}
	return name, nil
}

func (s *Service) backupFiles(destination string) ([]string, error) {
	entries, err := os.ReadDir(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && s.isBackupName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (s *Service) ImportManagedBackup(ctx context.Context, name string) (instance.ArchiveInstance, error) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	if !safeArchiveID(name) || !s.isBackupName(name) {
		return instance.ArchiveInstance{}, errors.New("select a managed backup")
	}
	status, err := s.readBackupStatus()
	if err != nil {
		return instance.ArchiveInstance{}, err
	}
	path := filepath.Join(status.Settings.Destination, name)
	info, err := os.Lstat(path)
	if err != nil {
		return instance.ArchiveInstance{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<30 {
		return instance.ArchiveInstance{}, errors.New("unsupported managed backup file")
	}
	input, err := os.Open(path)
	if err != nil {
		return instance.ArchiveInstance{}, err
	}
	defer input.Close()
	return s.Import(ctx, input, info.Size())
}

func (s *Service) pruneBackups(settings instance.BackupSettings) ([]string, error) {
	names, err := s.backupFiles(settings.Destination)
	if err != nil {
		return nil, err
	}
	var removed []string
	for len(names) > settings.Retain {
		name := names[0]
		if err := os.Remove(filepath.Join(settings.Destination, name)); err != nil {
			return removed, fmt.Errorf("remove expired backup: %w", err)
		}
		removed = append(removed, name)
		names = names[1:]
	}
	return removed, nil
}

func (s *Service) backupPrefix() string {
	digest := sha256.Sum256([]byte(s.root))
	return fmt.Sprintf("akoflow-backup-%x-", digest[:8])
}

func (s *Service) isBackupName(name string) bool {
	name, owned := strings.CutPrefix(name, s.backupPrefix())
	stamp, id, ok := strings.Cut(name, "-snapshot-")
	if !owned || !ok || !strings.HasSuffix(id, ".zip") {
		return false
	}
	_, err := time.Parse("20060102T150405.000000000Z", stamp)
	return err == nil
}

func (s *Service) writeBackupStatus(status instance.BackupStatus) error {
	return writeAtomic(s.root, ".backup-status.json", func(file *os.File) error {
		return json.NewEncoder(file).Encode(status)
	})
}

func writeAtomic(directory, name string, write func(*os.File) error) error {
	file, err := os.CreateTemp(directory, ".akoflow-writing-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := write(file); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(directory, name))
}

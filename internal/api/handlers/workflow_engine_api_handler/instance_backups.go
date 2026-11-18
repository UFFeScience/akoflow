package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/domain/instance"
)

type InstanceBackups interface {
	BackupStatus() (instance.BackupStatus, error)
	ConfigureBackups(instance.BackupSettings) (instance.BackupStatus, error)
	CreateBackup(context.Context) (instance.BackupStatus, error)
	ImportManagedBackup(context.Context, string) (instance.ArchiveInstance, error)
	LastRestore() (*instance.RestoreResult, error)
	PreviewRestore(context.Context, string) (instance.RestorePreview, error)
	ScheduleRestore(context.Context, string, string) error
}

func (h *Handler) GetBackups(w http.ResponseWriter, _ *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	status, err := h.instanceBackups.BackupStatus()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) ConfigureBackups(w http.ResponseWriter, r *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	var settings instance.BackupSettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	status, err := h.instanceBackups.ConfigureBackups(settings)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	status, err := h.instanceBackups.CreateBackup(r.Context())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, status)
}

func (h *Handler) backupsAvailable(w http.ResponseWriter) bool {
	if h.instanceBackups != nil {
		return true
	}
	writeError(w, http.StatusServiceUnavailable, errors.New("instance backups are unavailable"))
	return false
}

func (h *Handler) GetRestoreResult(w http.ResponseWriter, _ *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	result, err := h.instanceBackups.LastRestore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (h *Handler) PreviewRestore(w http.ResponseWriter, r *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	result, err := h.instanceBackups.PreviewRestore(r.Context(), r.PathValue("instanceId"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ScheduleRestore(w http.ResponseWriter, r *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	var body struct {
		Confirm  bool   `json:"confirmReplacement"`
		Checksum string `json:"checksum"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !body.Confirm || body.Checksum == "" {
		writeError(w, http.StatusBadRequest, errors.New("preview the archive and confirm replacement"))
		return
	}
	if err := h.instanceBackups.ScheduleRestore(r.Context(), r.PathValue("instanceId"), body.Checksum); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"restarting": h.restart != nil})
	if h.restart != nil {
		go h.restart()
	}
}

func (h *Handler) ImportManagedBackup(w http.ResponseWriter, r *http.Request) {
	if !h.backupsAvailable(w) {
		return
	}
	var body struct {
		File string `json:"file"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := h.instanceBackups.ImportManagedBackup(r.Context(), body.File)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

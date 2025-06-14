package internal_storage_handler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/storages_repository"
	file_list_parser_service "github.com/ovvesley/akoflow/pkg/server/services/file_disk_parser_service"
	"github.com/ovvesley/akoflow/pkg/server/services/file_spec_parser_service"
)

type InternalStorageHandler struct {
	fileSpecParserService file_spec_parser_service.FileSpecParserService
	fileListParserService file_list_parser_service.FileListParserService
	storageRepository     storages_repository.IStorageRepository
}

func (h *InternalStorageHandler) readText(r *http.Request) string {
	rawText, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	return string(rawText)
}

func New() *InternalStorageHandler {
	return &InternalStorageHandler{
		fileSpecParserService: file_spec_parser_service.New(),
		fileListParserService: file_list_parser_service.New(),
		storageRepository:     config.App().Repository.StoragesRepository,
	}
}

func (h *InternalStorageHandler) InitialFileListHandler(w http.ResponseWriter, r *http.Request) {

	activityIdStr := r.URL.Query().Get("activityId")
	activityId, _ := strconv.Atoi(activityIdStr)

	rawText := h.readText(r)

	fileDisk := h.fileListParserService.Parse(rawText)
	_ = h.storageRepository.UpdateInitialFileListDisk(activityId, fileDisk)

	_, _ = w.Write([]byte("ok"))
}

func (h *InternalStorageHandler) EndFileListHandler(w http.ResponseWriter, r *http.Request) {
	activityIdStr := r.URL.Query().Get("activityId")
	activityId, _ := strconv.Atoi(activityIdStr)

	rawText := h.readText(r)

	fileDisk := h.fileListParserService.Parse(rawText)
	_ = h.storageRepository.UpdateEndFileListDisk(activityId, fileDisk)

	_, _ = w.Write([]byte("ok"))
}

func (h *InternalStorageHandler) InitialDiskSpecHandler(w http.ResponseWriter, r *http.Request) {
	activityIdStr := r.URL.Query().Get("activityId")
	activityId, _ := strconv.Atoi(activityIdStr)

	rawText := h.readText(r)

	fileSpec := h.fileSpecParserService.Parse(rawText)
	_ = h.storageRepository.UpdateInitialDiskSpec(activityId, fileSpec)

	_, _ = w.Write([]byte("ok"))
}

func (h *InternalStorageHandler) EndDiskSpecHandler(w http.ResponseWriter, r *http.Request) {
	activityIdStr := r.URL.Query().Get("activityId")
	activityId, _ := strconv.Atoi(activityIdStr)

	rawText := h.readText(r)

	fileSpec := h.fileSpecParserService.Parse(rawText)
	_ = h.storageRepository.UpdateEndDiskSpec(activityId, fileSpec)

	_, _ = w.Write([]byte("ok"))
}

func (h *InternalStorageHandler) GetInitialFileListHandler(w http.ResponseWriter, r *http.Request) {
	activityIdStr := r.URL.Query().Get("activityId")
	activityId, _ := strconv.Atoi(activityIdStr)
	activity, err := h.storageRepository.Find(activityId)
	if err != nil {
		http.Error(w, "Error retrieving initial file list", http.StatusInternalServerError)
		return
	}
	// Crie uma struct anônima para resposta
	response := struct {
		InitialFileList string `json:"initial_file_list"`
		EndFileList     string `json:"end_file_list"`
		InitialDiskSpec string `json:"initial_disk_spec"`
		EndDiskSpec     string `json:"end_disk_spec"`
	}{
		InitialFileList: activity.InitialFileList,
		EndFileList:     activity.EndFileList,
		InitialDiskSpec: activity.InitialDiskSpec,
		EndDiskSpec:     activity.EndDiskSpec,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

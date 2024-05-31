package internal_storage_handler

import (
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/file_disk_parser_service"
	"github.com/ovvesley/akoflow/pkg/server/services/file_spec_parser_service"
	"io/ioutil"
	"net/http"
	"strconv"
)

type InternalStorageHandler struct {
	fileSpecParserService *file_spec_parser_service.FileSpecParserService
	fileListParserService *file_list_parser_service.FileListParserService
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
		storageRepository:     storages_repository.New(),
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

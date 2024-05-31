package storage_databasedump_handler

import (
	"net/http"
	"os"
)

type StorageDatabaseDumpHandler struct {
}

var PATH = "storage/database.db"

func New() *StorageDatabaseDumpHandler {
	return &StorageDatabaseDumpHandler{}
}

func (h *StorageDatabaseDumpHandler) DatabaseDumpHandler(w http.ResponseWriter, r *http.Request) {
	if !h.fileDumpExists() {
		_, _ = w.Write([]byte("{\"status\": \"error\", \"message\": \"File not found\"}"))
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=database.db")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", "0")
	
	http.ServeFile(w, r, PATH)

}

func (h *StorageDatabaseDumpHandler) fileDumpExists() bool {

	_, err := os.Stat(PATH)

	if err != nil {
		return false
	}

	return true
}

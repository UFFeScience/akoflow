package http_helper

import (
	"encoding/json"
	"net/http"
	"time"
)

// WriteJson writes a JSON response to the http.ResponseWriter
// {"data": data}
func WriteJson(w http.ResponseWriter, data interface{}) {
	current := map[string]interface{}{
		"data":      data,
		"timestamp": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(current)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}

package http_helper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unsafe"
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

func GetUrlPathParam(r *http.Request, key string) string {
	pattern := GetPatternFromRequest(r)
	if pattern == "" {
		return ""
	}

	pattern = strings.TrimSpace(strings.SplitN(pattern, " ", 2)[1])

	patternParts := strings.Split(pattern, "/")
	urlParts := strings.Split(r.URL.Path, "/")

	if len(patternParts) != len(urlParts) {
		return ""
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := part[1 : len(part)-1]
			if paramName == key {
				return urlParts[i]
			}
		}
	}

	return ""
}

func GetPatternFromRequest(r *http.Request) string {
	value := reflect.ValueOf(r).Elem()
	patternField := value.FieldByName("pat")
	if !patternField.IsValid() {
		return ""
	}

	pattern := reflect.NewAt(patternField.Type(), unsafe.Pointer(patternField.UnsafeAddr())).Elem().Interface()

	patternStr := fmt.Sprintf("%v", pattern)

	return patternStr
}

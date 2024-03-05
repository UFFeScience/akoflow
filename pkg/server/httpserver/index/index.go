package index

import "net/http"

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("K8s Scientific Workflow"))
	if err != nil {
		return
	}
}

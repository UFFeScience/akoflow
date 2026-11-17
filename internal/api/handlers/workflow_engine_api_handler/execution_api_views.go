package workflow_engine_api_handler

import "net/http"

func (h *Handler) GetExecutionStatus(w http.ResponseWriter, r *http.Request) {
	h.executionAPIView(w, r, "status")
}
func (h *Handler) GetExecutionLogs(w http.ResponseWriter, r *http.Request) {
	h.executionAPIView(w, r, "logs")
}
func (h *Handler) GetExecutionArtifacts(w http.ResponseWriter, r *http.Request) {
	h.executionAPIView(w, r, "data")
}
func (h *Handler) GetExecutionActivity(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	query.Set("activityId", r.PathValue("activityId"))
	r.URL.RawQuery = query.Encode()
	h.executionAPIView(w, r, "activity")
}
func (h *Handler) GetExecutionEvents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !query.Has("page") {
		query.Set("page", "1")
	}
	if !query.Has("pageSize") {
		query.Set("pageSize", "100")
	}
	r.URL.RawQuery = query.Encode()
	h.executionAPIView(w, r, "events")
}
func (h *Handler) executionAPIView(w http.ResponseWriter, r *http.Request, view string) {
	query := r.URL.Query()
	query.Set("view", view)
	r.URL.RawQuery = query.Encode()
	h.GetExecution(w, r)
}

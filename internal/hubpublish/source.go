package hubpublish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type snapshotResponse struct {
	header   http.Header
	status   int
	data     bytes.Buffer
	overflow bool
}

func (w *snapshotResponse) Header() http.Header { return w.header }
func (w *snapshotResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *snapshotResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	if w.data.Len()+len(data) > 24<<20 {
		w.overflow = true
		return 0, errors.New("snapshot response limit exceeded")
	}
	return w.data.Write(data)
}
func (m *Manager) readSource(ctx context.Context, path string, output any) error {
	request, err := http.NewRequestWithContext(ctx, "GET", "http://akoflow.internal/akoflow-api"+path, nil)
	if err != nil {
		return failure(422, "source_invalid", "Choose a valid workflow execution.")
	}
	response := &snapshotResponse{header: http.Header{}}
	m.options.Source.ServeHTTP(response, request)
	if response.status != 200 || response.overflow {
		return failure(422, "source_unavailable", "This execution or its immutable planning context is unavailable.")
	}
	if json.Unmarshal(response.data.Bytes(), output) != nil {
		return failure(502, "source_invalid", "The daemon returned an invalid execution snapshot.")
	}
	return nil
}

package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/hubtelemetry"
)

func telemetryAuditStore(ctx context.Context, store ports.AuditStore, routes *http.ServeMux, security SecurityOptions) ports.AuditStore {
	token := os.Getenv("AKOFLOW_TELEMETRY_TOKEN")
	endpoint := os.Getenv("AKOFLOW_TELEMETRY_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://hub.akoflow.com/api/v1/telemetry/events"
	}
	sender := hubtelemetry.New(endpoint, token)
	sender.SetConsent(hubtelemetry.Diagnostics, os.Getenv("AKOFLOW_TELEMETRY_DIAGNOSTICS") == "true")
	sender.SetConsent(hubtelemetry.APIAccess, os.Getenv("AKOFLOW_TELEMETRY_API_ACCESS") == "true")
	routes.HandleFunc("PUT /akoflow-api/telemetry/consent", telemetryConsent(ctx, sender, security, 15*time.Second))
	go sender.Run(ctx)
	return hubtelemetry.AuditStore{AuditStore: store, Sender: sender, Release: config.GetVersion()}
}

func telemetryConsent(ctx context.Context, sender *hubtelemetry.Sender, security SecurityOptions, lease time.Duration) http.HandlerFunc {
	var mu sync.Mutex
	var expiration *time.Timer
	var generation uint64
	context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		if expiration != nil {
			expiration.Stop()
		}
	})
	return func(w http.ResponseWriter, r *http.Request) {
		if security.BearerToken == "" || !validBearer(r, security.BearerToken) ||
			r.Header.Get("X-Akoflow-Native-Request") != "telemetry-consent" {
			http.Error(w, "native authentication required", http.StatusForbidden)
			return
		}
		var choice struct {
			Diagnostics *bool `json:"diagnostics"`
			APIAccess   *bool `json:"apiAccess"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&choice) != nil || decoder.Decode(new(any)) != io.EOF ||
			choice.Diagnostics == nil || choice.APIAccess == nil {
			http.Error(w, "explicit consent is required for each category", http.StatusBadRequest)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if ctx.Err() != nil {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		if expiration != nil {
			expiration.Stop()
		}
		generation++
		current := generation
		if *choice.Diagnostics || *choice.APIAccess {
			expiration = time.AfterFunc(lease, func() {
				mu.Lock()
				defer mu.Unlock()
				if generation == current {
					sender.SetConsent(hubtelemetry.Diagnostics, false)
					sender.SetConsent(hubtelemetry.APIAccess, false)
				}
			})
		}
		sender.SetConsent(hubtelemetry.Diagnostics, *choice.Diagnostics)
		sender.SetConsent(hubtelemetry.APIAccess, *choice.APIAccess)
		w.WriteHeader(http.StatusNoContent)
	}
}

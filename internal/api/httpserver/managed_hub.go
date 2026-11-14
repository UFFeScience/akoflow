package httpserver

import (
	"github.com/UFFeScience/akoflow/internal/hubpublish"
	"github.com/UFFeScience/akoflow/internal/infrastructure/instancearchive"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func managedHubRoutes(next http.Handler, source http.Handler, security SecurityOptions, readOnly bool) (http.Handler, error) {
	directory := filepath.Join(filepath.Dir(instancearchive.ResolveDatabasePath()), "credentials", "hub")
	manager, err := hubpublish.New(hubpublish.Options{Directory: directory, HubURL: os.Getenv("AKOFLOW_HUB_URL"), BrowserURL: os.Getenv("AKOFLOW_HUB_BROWSER_URL"), Source: source, ReadOnly: readOnly})
	if err != nil {
		return nil, err
	}
	controls := AllowCORSFor(SecureAPI(manager.Handler(), security), security.AllowedOrigins)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/akoflow-api/hub/") {
			controls.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

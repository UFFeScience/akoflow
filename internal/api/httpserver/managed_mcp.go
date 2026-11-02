package httpserver

import (
	"net"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/instancearchive"
	"github.com/UFFeScience/akoflow/internal/mcpcontrol"
)

func managedMCPRoutes(next http.Handler, address string, security SecurityOptions, settings config.Settings) (http.Handler, func(), error) {
	var legacy http.Handler
	closeLegacy := func() {}
	if settings.MCPEnabled {
		var err error
		legacy, closeLegacy, err = newMCPHandler(address, security, settings)
		if err != nil {
			return nil, nil, err
		}
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		closeLegacy()
		return nil, nil, err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	directory := mcpcontrol.StateDirectory(settings.MCPStateDirectory, instancearchive.ResolveDatabasePath())
	manager, err := mcpcontrol.New(mcpcontrol.Options{
		APIURL:   "http://" + net.JoinHostPort(host, port) + "/akoflow-api",
		APIToken: security.BearerToken, Directory: directory,
		InitiallyEnabled: settings.MCPEnabled, Legacy: legacy,
	})
	if err != nil {
		closeLegacy()
		return nil, nil, err
	}
	// Installation access is independent of the selected archive. Domain requests
	// still pass through readOnlyAPI and keep the archive's mutation restrictions.
	controls := AllowCORSFor(SecureAPI(manager.ControlHandler(), security), security.AllowedOrigins)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/akoflow-api/mcp" || len(r.URL.Path) >= len("/akoflow-api/mcp/") && r.URL.Path[:len("/akoflow-api/mcp/")] == "/akoflow-api/mcp/" {
			controls.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/mcp" || r.URL.Path == "/.well-known/oauth-protected-resource" || r.URL.Path == "/.well-known/oauth-protected-resource/mcp" {
			manager.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
	return handler, func() { manager.Close(); closeLegacy() }, nil
}

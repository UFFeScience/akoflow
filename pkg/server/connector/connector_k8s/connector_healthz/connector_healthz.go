package connector_healthz

import (
	"crypto/tls"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorHealthz struct {
	runtime *runtime_entity.Runtime
}

type IConnectorHealthz interface {
	Healthz() ResultHealthz
}

func New(runtime *runtime_entity.Runtime) IConnectorHealthz {
	return &ConnectorHealthz{
		runtime: runtime,
	}
}

// Helper function to create a new HTTP client
func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

type ResultHealthz struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *ConnectorHealthz) Healthz() ResultHealthz {
	// Create a new HTTP client
	client := newClient()

	// Create a new request to the healthz endpoint
	req, err := http.NewRequest("GET", "https://"+c.runtime.GetMetadataApiServerHost()+"/healthz", nil)
	if err != nil {
		return ResultHealthz{
			Success: false,
			Message: err.Error(),
		}
	}

	// Set the authorization header with the token
	req.Header.Set("Authorization", "Bearer "+c.runtime.GetMetadataApiServerToken())

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return ResultHealthz{
			Success: false,
			Message: err.Error(),
		}
	}
	defer resp.Body.Close()

	return ResultHealthz{
		Success: resp.StatusCode == http.StatusOK,
		Message: "",
		Data:    nil,
	}
}

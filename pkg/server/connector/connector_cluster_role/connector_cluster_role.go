package connector_cluster_role

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ovvesley/akoflow/pkg/server/entities/nfs_server_entity"
)

type ConnectorClusterRole struct {
	client *http.Client
}

type IConnectorClusterRole interface {
	CreateClusterRole(clusterRole nfs_server_entity.ClusterRole) ResultCreateClusterRole
}

func New() IConnectorClusterRole {
	return &ConnectorClusterRole{
		client: newClient(),
	}
}

func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

type ResultCreateClusterRole struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *ConnectorClusterRole) CreateClusterRole(clusterRole nfs_server_entity.ClusterRole) ResultCreateClusterRole {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&clusterRole)
	if err != nil {
		log.Printf("Error marshaling cluster role: %s", err.Error())
		return ResultCreateClusterRole{
			Success: false,
			Message: fmt.Sprintf("Error marshaling cluster role: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/apis/rbac.authorization.k8s.io/v1/clusterroles", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateClusterRole{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateClusterRole{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating ClusterRole: %s", body.String())
		return ResultCreateClusterRole{
			Success: false,
			Message: fmt.Sprintf("Error creating ClusterRole: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateClusterRole{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateClusterRole{
		Success: true,
		Message: "ClusterRole created successfully",
		Data:    result,
	}
}

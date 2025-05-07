package connector_cluster_role_binding

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/entities/nfs_server_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorClusterRoleBinding struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnectorClusterRoleBinding interface {
	CreateClusterRoleBinding(clusterRoleBinding nfs_server_entity.ClusterRoleBinding) ResultCreateClusterRoleBinding
}

func New(runtime *runtime_entity.Runtime) IConnectorClusterRoleBinding {
	return &ConnectorClusterRoleBinding{
		client:  newClient(),
		runtime: runtime,
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

type ResultCreateClusterRoleBinding struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *ConnectorClusterRoleBinding) CreateClusterRoleBinding(clusterRoleBinding nfs_server_entity.ClusterRoleBinding) ResultCreateClusterRoleBinding {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	body, err := json.Marshal(&clusterRoleBinding)
	if err != nil {
		log.Printf("Error marshaling cluster role binding: %s", err.Error())
		return ResultCreateClusterRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error marshaling cluster role binding: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateClusterRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateClusterRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating ClusterRoleBinding: %s", body.String())
		return ResultCreateClusterRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error creating ClusterRoleBinding: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateClusterRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateClusterRoleBinding{
		Success: true,
		Message: "ClusterRoleBinding created successfully",
		Data:    result,
	}
}

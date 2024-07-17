package connector_role_binding

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

type ConnectorRoleBinding struct {
	client *http.Client
}

type IConnectorRoleBinding interface {
	CreateRoleBinding(roleBinding nfs_server_entity.RoleBinding) ResultCreateRoleBinding
}

func New() IConnectorRoleBinding {
	return &ConnectorRoleBinding{
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

type ResultCreateRoleBinding struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *ConnectorRoleBinding) CreateRoleBinding(roleBinding nfs_server_entity.RoleBinding) ResultCreateRoleBinding {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&roleBinding)
	if err != nil {
		log.Printf("Error marshaling role binding: %s", err.Error())
		return ResultCreateRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error marshaling role binding: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/apis/rbac.authorization.k8s.io/v1/namespaces/"+roleBinding.Metadata.Namespace+"/rolebindings", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating RoleBinding: %s", body.String())
		return ResultCreateRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error creating RoleBinding: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateRoleBinding{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateRoleBinding{
		Success: true,
		Message: "RoleBinding created successfully",
		Data:    result,
	}
}

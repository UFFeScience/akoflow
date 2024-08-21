package connector_role

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

type ConnectorRole struct {
	client *http.Client
}

type IConnectorRole interface {
	CreateRole(role nfs_server_entity.Role) ResultCreateRole
}

func New() IConnectorRole {
	return &ConnectorRole{
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

type ResultCreateRole struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *ConnectorRole) CreateRole(role nfs_server_entity.Role) ResultCreateRole {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&role)
	if err != nil {
		log.Printf("Error marshaling role: %s", err.Error())
		return ResultCreateRole{
			Success: false,
			Message: fmt.Sprintf("Error marshaling role: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/apis/rbac.authorization.k8s.io/v1/namespaces/"+role.Metadata.Namespace+"/roles", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateRole{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateRole{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating Role: %s", body.String())
		return ResultCreateRole{
			Success: false,
			Message: fmt.Sprintf("Error creating Role: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateRole{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateRole{
		Success: true,
		Message: "Role created successfully",
		Data:    result,
	}
}

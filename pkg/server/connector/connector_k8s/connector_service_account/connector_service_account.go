package connector_service_account

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

type ConnectorServiceAccount struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnectorServiceAccount interface {
	CreateServiceAccount(serviceAccount nfs_server_entity.ServiceAccount) ResultCreateServiceAccount
	ListServiceAccount(namespace string) ResultListServiceAccount
	UpdateServiceAccount(serviceAccount nfs_server_entity.ServiceAccount) ResultUpdateServiceAccount
	DeleteServiceAccount(namespace, name string) ResultDeleteServiceAccount
}

func New(runtime *runtime_entity.Runtime) IConnectorServiceAccount {
	return &ConnectorServiceAccount{
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

type ResultCreateServiceAccount struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultListServiceAccount struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultUpdateServiceAccount struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultDeleteServiceAccount struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (c *ConnectorServiceAccount) CreateServiceAccount(serviceAccount nfs_server_entity.ServiceAccount) ResultCreateServiceAccount {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	body, err := json.Marshal(&serviceAccount)
	if err != nil {
		log.Printf("Error marshaling service account: %s", err.Error())
		return ResultCreateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error marshaling service account: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/api/v1/namespaces/"+serviceAccount.Metadata.Namespace+"/serviceaccounts", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating ServiceAccount: %s", body.String())
		return ResultCreateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error creating ServiceAccount: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateServiceAccount{
		Success: true,
		Message: "ServiceAccount created successfully",
		Data:    result,
	}
}

func (c *ConnectorServiceAccount) ListServiceAccount(namespace string) ResultListServiceAccount {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, err := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/serviceaccounts", nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultListServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultListServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error listing ServiceAccounts: %s", body.String())
		return ResultListServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error listing ServiceAccounts: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultListServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultListServiceAccount{
		Success: true,
		Message: "ServiceAccounts listed successfully",
		Data:    result,
	}
}

func (c *ConnectorServiceAccount) UpdateServiceAccount(serviceAccount nfs_server_entity.ServiceAccount) ResultUpdateServiceAccount {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	body, err := json.Marshal(&serviceAccount)
	if err != nil {
		log.Printf("Error marshaling service account: %s", err.Error())
		return ResultUpdateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error marshaling service account: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("PUT", "https://"+host+"/api/v1/namespaces/"+serviceAccount.Metadata.Namespace+"/serviceaccounts/"+serviceAccount.Metadata.Name, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultUpdateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultUpdateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error updating ServiceAccount: %s", body.String())
		return ResultUpdateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error updating ServiceAccount: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultUpdateServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultUpdateServiceAccount{
		Success: true,
		Message: "ServiceAccount updated successfully",
		Data:    result,
	}
}

func (c *ConnectorServiceAccount) DeleteServiceAccount(namespace, name string) ResultDeleteServiceAccount {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, err := http.NewRequest("DELETE", "https://"+host+"/api/v1/namespaces/"+namespace+"/serviceaccounts/"+name, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultDeleteServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultDeleteServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error deleting ServiceAccount: %s", body.String())
		return ResultDeleteServiceAccount{
			Success: false,
			Message: fmt.Sprintf("Error deleting ServiceAccount: %s", body.String()),
		}
	}

	return ResultDeleteServiceAccount{
		Success: true,
		Message: "ServiceAccount deleted successfully",
	}
}

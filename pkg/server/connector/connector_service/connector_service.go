package connector_service

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

type ConnectorService struct {
	client *http.Client
}

type IConnectorService interface {
	CreateService(service nfs_server_entity.Service) ResultCreateService
	ListService(namespace string) ResultListService
	UpdateService(service nfs_server_entity.Service) ResultUpdateService
	DeleteService(namespace, name string) ResultDeleteService
}

func New() IConnectorService {
	return &ConnectorService{
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

type ResultCreateService struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultListService struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultUpdateService struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultDeleteService struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (c *ConnectorService) CreateService(service nfs_server_entity.Service) ResultCreateService {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&service)
	if err != nil {
		log.Printf("Error marshaling service: %s", err.Error())
		return ResultCreateService{
			Success: false,
			Message: fmt.Sprintf("Error marshaling service: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/api/v1/namespaces/"+service.Metadata.Namespace+"/services", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateService{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateService{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating Service: %s", body.String())
		return ResultCreateService{
			Success: false,
			Message: fmt.Sprintf("Error creating Service: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateService{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateService{
		Success: true,
		Message: "Service created successfully",
		Data:    result,
	}
}

func (c *ConnectorService) ListService(namespace string) ResultListService {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, err := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/services", nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultListService{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultListService{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error listing Services: %s", body.String())
		return ResultListService{
			Success: false,
			Message: fmt.Sprintf("Error listing Services: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultListService{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultListService{
		Success: true,
		Message: "Services listed successfully",
		Data:    result,
	}
}

func (c *ConnectorService) UpdateService(service nfs_server_entity.Service) ResultUpdateService {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&service)
	if err != nil {
		log.Printf("Error marshaling service: %s", err.Error())
		return ResultUpdateService{
			Success: false,
			Message: fmt.Sprintf("Error marshaling service: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("PUT", "https://"+host+"/api/v1/namespaces/"+service.Metadata.Namespace+"/services/"+service.Metadata.Name, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultUpdateService{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultUpdateService{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error updating Service: %s", body.String())
		return ResultUpdateService{
			Success: false,
			Message: fmt.Sprintf("Error updating Service: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultUpdateService{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultUpdateService{
		Success: true,
		Message: "Service updated successfully",
		Data:    result,
	}
}

func (c *ConnectorService) DeleteService(namespace, name string) ResultDeleteService {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, err := http.NewRequest("DELETE", "https://"+host+"/api/v1/namespaces/"+namespace+"/services/"+name, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultDeleteService{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultDeleteService{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error deleting Service: %s", body.String())
		return ResultDeleteService{
			Success: false,
			Message: fmt.Sprintf("Error deleting Service: %s", body.String()),
		}
	}

	return ResultDeleteService{
		Success: true,
		Message: "Service deleted successfully",
	}
}

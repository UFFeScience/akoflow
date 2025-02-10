package connector_storage_class

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

type ConnectorStorageClass struct {
	client *http.Client
}

type IConnectorStorageClass interface {
	CreateStorageClass(storageClass nfs_server_entity.StorageClass) ResultCreateStorageClass
	ListStorageClass() ResultListStorageClass
	UpdateStorageClass(storageClass nfs_server_entity.StorageClass) ResultUpdateStorageClass
	DeleteStorageClass(name string) ResultDeleteStorageClass
}

func New() IConnectorStorageClass {
	return &ConnectorStorageClass{
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

type ResultCreateStorageClass struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultListStorageClass struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultUpdateStorageClass struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultDeleteStorageClass struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (c *ConnectorStorageClass) CreateStorageClass(storageClass nfs_server_entity.StorageClass) ResultCreateStorageClass {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&storageClass)
	if err != nil {
		log.Printf("Error marshaling storage class: %s", err.Error())
		return ResultCreateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error marshaling storage class: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/apis/storage.k8s.io/v1/storageclasses", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating StorageClass: %s", body.String())
		return ResultCreateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error creating StorageClass: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateStorageClass{
		Success: true,
		Message: "StorageClass created successfully",
		Data:    result,
	}
}

func (c *ConnectorStorageClass) ListStorageClass() ResultListStorageClass {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, err := http.NewRequest("GET", "https://"+host+"/apis/storage.k8s.io/v1/storageclasses", nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultListStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultListStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error listing StorageClasses: %s", body.String())
		return ResultListStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error listing StorageClasses: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultListStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultListStorageClass{
		Success: true,
		Message: "StorageClasses listed successfully",
		Data:    result,
	}
}

func (c *ConnectorStorageClass) UpdateStorageClass(storageClass nfs_server_entity.StorageClass) ResultUpdateStorageClass {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&storageClass)
	if err != nil {
		log.Printf("Error marshaling storage class: %s", err.Error())
		return ResultUpdateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error marshaling storage class: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("PUT", "https://"+host+"/apis/storage.k8s.io/v1/storageclasses/"+storageClass.Metadata.Name, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultUpdateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultUpdateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error updating StorageClass: %s", body.String())
		return ResultUpdateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error updating StorageClass: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultUpdateStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultUpdateStorageClass{
		Success: true,
		Message: "StorageClass updated successfully",
		Data:    result,
	}
}

func (c *ConnectorStorageClass) DeleteStorageClass(name string) ResultDeleteStorageClass {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, err := http.NewRequest("DELETE", "https://"+host+"/apis/storage.k8s.io/v1/storageclasses/"+name, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultDeleteStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultDeleteStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error deleting StorageClass: %s", body.String())
		return ResultDeleteStorageClass{
			Success: false,
			Message: fmt.Sprintf("Error deleting StorageClass: %s", body.String()),
		}
	}

	return ResultDeleteStorageClass{
		Success: true,
		Message: "StorageClass deleted successfully",
	}
}

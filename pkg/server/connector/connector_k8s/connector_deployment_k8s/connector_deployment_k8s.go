package connector_deployment_k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ovvesley/akoflow/pkg/server/entities/nfs_server_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorDeploymentK8s struct {
	client *http.Client
}

type IConnectorDeployment interface {
	ListDeployments(namespace string) ResultListDeployment
	CreateDeployment(deployment nfs_server_entity.Deployment) ResultCreateDeployment
	UpdateDeployment(deployment nfs_server_entity.Deployment) ResultUpdateDeployment
	DeleteDeployment(namespace, name string) ResultDeleteDeployment
	GetDeployment(namespace, deploymentName string) ResultGetDeployment
}

func New(*runtime_entity.Runtime) IConnectorDeployment {
	return &ConnectorDeploymentK8s{
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

type ResultListDeployment struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultCreateDeployment struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultUpdateDeployment struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ResultDeleteDeployment struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ResultGetDeployment struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *ConnectorDeploymentK8s) ListDeployments(namespace string) ResultListDeployment {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, err := http.NewRequest("GET", "https://"+host+"/apis/apps/v1/namespaces/"+namespace+"/deployments", nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultListDeployment{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultListDeployment{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error listing Deployments: %s", body.String())
		return ResultListDeployment{
			Success: false,
			Message: fmt.Sprintf("Error listing Deployments: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultListDeployment{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultListDeployment{
		Success: true,
		Message: "Deployments listed successfully",
		Data:    result,
	}
}

func (c *ConnectorDeploymentK8s) GetDeployment(namespace, deploymentName string) ResultGetDeployment {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	url := fmt.Sprintf("https://%s/apis/apps/v1/namespaces/%s/deployments/%s", host, namespace, deploymentName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultGetDeployment{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultGetDeployment{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error getting Deployment: %s", body.String())
		return ResultGetDeployment{
			Success: false,
			Message: fmt.Sprintf("Error getting Deployment: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultGetDeployment{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultGetDeployment{
		Success: true,
		Message: "Deployment retrieved successfully",
		Data:    result,
	}
}

func (c *ConnectorDeploymentK8s) CreateDeployment(deployment nfs_server_entity.Deployment) ResultCreateDeployment {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&deployment)
	if err != nil {
		log.Printf("Error marshaling deployment: %s", err.Error())
		return ResultCreateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error marshaling deployment: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("POST", "https://"+host+"/apis/apps/v1/namespaces/"+deployment.Metadata.Namespace+"/deployments", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultCreateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultCreateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error creating Deployment: %s", body.String())
		return ResultCreateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error creating Deployment: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultCreateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultCreateDeployment{
		Success: true,
		Message: "Deployment created successfully",
		Data:    result,
	}
}

func (c *ConnectorDeploymentK8s) UpdateDeployment(deployment nfs_server_entity.Deployment) ResultUpdateDeployment {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, err := json.Marshal(&deployment)
	if err != nil {
		log.Printf("Error marshaling deployment: %s", err.Error())
		return ResultUpdateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error marshaling deployment: %s", err.Error()),
		}
	}

	req, err := http.NewRequest("PUT", "https://"+host+"/apis/apps/v1/namespaces/"+deployment.Metadata.Namespace+"/deployments/"+deployment.Metadata.Name, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultUpdateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultUpdateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error updating Deployment: %s", body.String())
		return ResultUpdateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error updating Deployment: %s", body.String()),
		}
	}

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return ResultUpdateDeployment{
			Success: false,
			Message: fmt.Sprintf("Error decoding response: %s", err.Error()),
		}
	}

	return ResultUpdateDeployment{
		Success: true,
		Message: "Deployment updated successfully",
		Data:    result,
	}
}

func (c *ConnectorDeploymentK8s) DeleteDeployment(namespace, name string) ResultDeleteDeployment {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, err := http.NewRequest("DELETE", "https://"+host+"/apis/apps/v1/namespaces/"+namespace+"/deployments/"+name, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %s", err.Error())
		return ResultDeleteDeployment{
			Success: false,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request: %s", err.Error())
		return ResultDeleteDeployment{
			Success: false,
			Message: fmt.Sprintf("Error making HTTP request: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Printf("Error deleting Deployment: %s", body.String())
		return ResultDeleteDeployment{
			Success: false,
			Message: fmt.Sprintf("Error deleting Deployment: %s", body.String()),
		}
	}

	return ResultDeleteDeployment{
		Success: true,
		Message: "Deployment deleted successfully",
	}
}

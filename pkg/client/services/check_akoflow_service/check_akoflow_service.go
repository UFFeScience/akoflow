package check_akoflow_service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

type CheckAkoflowService struct {
	host     string
	port     string
	services []string
}

type ServiceStatusRequest struct {
	Service string `json:"service"`
}

type ServiceStatusResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func New() *CheckAkoflowService {
	return &CheckAkoflowService{}
}

func (c *CheckAkoflowService) SetHost(host string) *CheckAkoflowService {
	c.host = host
	return c
}

func (c *CheckAkoflowService) SetPort(port string) *CheckAkoflowService {
	c.port = port
	return c
}

func (c *CheckAkoflowService) SetServices(services []string) *CheckAkoflowService {
	c.services = services
	return c
}

func (c *CheckAkoflowService) GetHost() string {
	return c.host
}

func (c *CheckAkoflowService) GetPort() string {
	return c.port
}

func (c *CheckAkoflowService) Run() {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	for _, service := range c.services {
		c.checkService(client, service)
	}
}

func (c *CheckAkoflowService) checkService(client *http.Client, serviceName string) {
	url := fmt.Sprintf("http://%s:%s/akoflow-server/check-service", c.GetHost(), c.GetPort())

	payload := ServiceStatusRequest{
		Service: serviceName,
	}

	payloadJson, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Service %s Failed: %v\n", serviceName, err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadJson))
	if err != nil {
		fmt.Printf("Service %s Failed: %v\n", serviceName, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Service %s Failed: %v\n", serviceName, err)
		return
	}
	defer resp.Body.Close()

	var result ServiceStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Printf("Service %s Failed: %v\n", serviceName, err)
		return
	}

	if result.Status == "OK" {
		fmt.Printf("Service %s OK\n", serviceName)
	} else {
		fmt.Printf("Service %s Failed: %s\n", serviceName, result.Message)
	}
}

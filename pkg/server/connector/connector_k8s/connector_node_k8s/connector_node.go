package connector_node_k8s

import (
	"crypto/tls"
	"encoding/json"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorNodeK8s struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnectorNodeK8s interface {
	ListNodes() ConnectorNodeK8sResponse
}

func New(runtime *runtime_entity.Runtime) IConnectorNodeK8s {
	return &ConnectorNodeK8s{
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

type Node struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Status     string `json:"status"`
	CpuMax     string `json:"cpu_max"`
	MemoryMax  string `json:"memory_max"`
	NetworkMax string `json:"network_max"`
	DiskMax    string `json:"disk_max"`
	OsImage    string `json:"os_image"`
	CreatedAt  string `json:"created_at"`
}

func (n Node) GetCpuMax() float64 {
	cpuMax := n.CpuMax
	if cpuMax == "" {
		return 0.0
	}
	var cpu float64
	if err := json.Unmarshal([]byte(cpuMax), &cpu); err != nil {
		return 0.0
	}
	return cpu
}

func (n Node) GetNodeMemoryMax() float64 {
	memoryMax := n.MemoryMax
	if memoryMax == "" {
		return 0.0
	}
	if len(memoryMax) > 2 && memoryMax[len(memoryMax)-2:] == "Ki" {
		memoryMax = memoryMax[:len(memoryMax)-2]
	}
	var memory float64
	if err := json.Unmarshal([]byte(memoryMax), &memory); err != nil {
		return 0.0
	}
	return memory / 1024.0
}

func (n Node) GetNodeNetworkMax() float64 {
	networkMax := n.NetworkMax
	if networkMax == "" {
		return 0.0
	}
	var network float64
	if err := json.Unmarshal([]byte(networkMax), &network); err != nil {
		return 0.0
	}
	return network
}

type ConnectorNodeK8sResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    []Node `json:"data,omitempty"`
}

func (c ConnectorNodeK8s) ListNodes() ConnectorNodeK8sResponse {
	host := c.runtime.GetMetadataApiServerHost()

	req, err := http.NewRequest("GET", "https://"+host+"/api/v1/nodes", nil)

	if err != nil {
		return ConnectorNodeK8sResponse{
			Success: false,
			Message: "Failed to create request: " + err.Error(),
		}
	}

	req.Header.Set("Authorization", "Bearer "+c.runtime.GetMetadataApiServerToken())

	resp, err := c.client.Do(req)
	if err != nil {
		return ConnectorNodeK8sResponse{
			Success: false,
			Message: "Failed to execute request: " + err.Error(),
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ConnectorNodeK8sResponse{
			Success: false,
			Message: "Failed to list nodes, status code: " + resp.Status,
		}
	}

	var data any

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ConnectorNodeK8sResponse{
			Success: false,
			Message: "Failed to decode response: " + err.Error(),
		}
	}

	var nodes []Node
	for _, item := range data.(map[string]interface{})["items"].([]interface{}) {
		name := item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)
		memoryMax := item.(map[string]interface{})["status"].(map[string]interface{})["allocatable"].(map[string]interface{})["memory"]
		cpuMax := item.(map[string]interface{})["status"].(map[string]interface{})["allocatable"].(map[string]interface{})["cpu"]
		networkMax := item.(map[string]interface{})["status"].(map[string]interface{})["allocatable"].(map[string]interface{})["ephemeral-storage"]
		DiskMax := item.(map[string]interface{})["status"].(map[string]interface{})["capacity"].(map[string]interface{})["ephemeral-storage"]
		osImage := item.(map[string]interface{})["status"].(map[string]interface{})["nodeInfo"].(map[string]interface{})["osImage"].(string)
		createdAt := item.(map[string]interface{})["metadata"].(map[string]interface{})["creationTimestamp"].(string)

		nodes = append(nodes, Node{
			Name:       name,
			Status:     "Ready", // Assuming all nodes are ready, adjust as needed
			CpuMax:     cpuMax.(string),
			MemoryMax:  memoryMax.(string),
			NetworkMax: networkMax.(string),
			DiskMax:    DiskMax.(string),
			OsImage:    osImage,
			CreatedAt:  createdAt,
		})

	}

	return ConnectorNodeK8sResponse{
		Success: true,
		Message: "Nodes listed successfully",
		Data:    nodes,
	}

}

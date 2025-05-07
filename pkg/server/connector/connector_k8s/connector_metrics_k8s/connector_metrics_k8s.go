package connector_metrics_k8s

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorMetricsK8s struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnectorMetrics interface {
	ListMetrics()
	GetPodMetrics(namespace string, podName string) (ResponseGetPodMetrics, error)
}

func New(runtime *runtime_entity.Runtime) IConnectorMetrics {
	return &ConnectorMetricsK8s{
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

func (c ConnectorMetricsK8s) ListMetrics() {
	//TODO implement me
	panic("implement me")
}

type ResponseGetPodMetrics struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			App             string `json:"app"`
			PodTemplateHash string `json:"pod-template-hash"`
		} `json:"labels"`
	} `json:"metadata"`
	Timestamp  time.Time `json:"timestamp"`
	Window     string    `json:"window"`
	Containers []struct {
		Name  string `json:"name"`
		Usage struct {
			Cpu    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"containers"`
}

type Metrics struct {
	ActivityId *int
	Window     string
	Timestamp  string
	Cpu        string `json:"cpu"`
	Memory     string `json:"memory"`
}

func (c ResponseGetPodMetrics) GetMetrics() (Metrics, error) {
	metrics := Metrics{}

	if len(c.Containers) == 0 {
		return metrics, errors.New("no metrics found")
	}

	metrics.Window = c.Window
	metrics.Timestamp = c.Timestamp.String()

	metrics.Cpu = c.Containers[0].Usage.Cpu
	metrics.Memory = c.Containers[0].Usage.Memory

	return metrics, nil
}

func (c *ConnectorMetricsK8s) GetPodMetrics(namespace string, podName string) (ResponseGetPodMetrics, error) {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, _ := http.NewRequest("GET", "https://"+host+"/apis/metrics.k8s.io/v1beta1/namespaces/"+namespace+"/pods/"+podName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseGetPodMetrics{}, err
	}

	if resp.StatusCode != 200 {
		return ResponseGetPodMetrics{}, fmt.Errorf("Metric server not found: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	var result ResponseGetPodMetrics

	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetPodMetrics{}, err
	}

	return result, nil

}

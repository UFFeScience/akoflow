package connector

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/k8sjob"
	"net/http"
	"os"
)

type Connector struct {
	client *http.Client
}

func New() *Connector {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	return &Connector{client: client}
}

func (c *Connector) ListNamespaces() interface{} {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, _ := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return nil
	}

	defer resp.Body.Close()

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return nil
	}

	return result
}

func (c *Connector) ApplyJob(namespace string, job k8sjob.K8sJob) interface{} {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	body, _ := json.Marshal(&job)
	fmt.Println(string(body))

	req, _ := http.NewRequest("POST", "https://"+host+"/apis/batch/v1/namespaces/"+namespace+"/jobs", bytes.NewBuffer(body))

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)

	if err != nil {
		return nil
	}

	defer resp.Body.Close()

	var result interface{}
	println(resp.StatusCode)
	println(string(body))
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return nil
	}

	return result

}

func (c *Connector) GetJob(namespace string, jobName string) (ResponseGetJob, error) {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, _ := http.NewRequest("GET", "https://"+host+"/apis/batch/v1/namespaces/"+namespace+"/jobs/"+jobName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseGetJob{}, err
	}

	defer resp.Body.Close()

	var result ResponseGetJob
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetJob{}, err
	}

	return result, nil
}

func (c *Connector) GetPodByJob(namespace string, jobName string) (ResponseGetJobByPod, error) {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, _ := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/pods?labelSelector=job-name="+jobName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseGetJobByPod{}, err
	}

	defer resp.Body.Close()

	var result ResponseGetJobByPod
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetJobByPod{}, err
	}

	return result, nil
}

func (c *Connector) GetPodMetrics(namespace string, podName string) (ResponseGetPodMetrics, error) {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, _ := http.NewRequest("GET", "https://"+host+"/apis/metrics.k8s.io/v1beta1/namespaces/"+namespace+"/pods/"+podName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseGetPodMetrics{}, err
	}

	defer resp.Body.Close()

	var result ResponseGetPodMetrics

	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetPodMetrics{}, err
	}

	return result, nil

}

func (c *Connector) GetPodLogs(namespace string, podName string) (string, error) {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, _ := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/pods/"+podName+"/log", nil)

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	buf.ReadFrom(resp.Body)

	return buf.String(), nil
}

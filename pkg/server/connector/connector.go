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

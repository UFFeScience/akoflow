package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

func main() {

	//URI := "http://workflow-controller-service:8080"

	//response := connector.New(URI).Connect()
	pod := os.Getenv("HOSTNAME")

	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("KUBERNETES_SERVICE_HOST")

	fmt.Println("Token: ", token)
	fmt.Println("Host: ", host)

	// get request to k8s api server with bearer token

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	for i := 0; i < 3600; i++ {
		println(fmt.Sprintf("%d seconds left - %s", 3600-i, pod))
		time.Sleep(1 * time.Second)

		req, err := http.NewRequest("GET", "https://"+host+":443/api/v1/namespaces/", nil)

		if err != nil {
			fmt.Println("Error: ", err)
		}

		req.Header.Add("Authorization", "Bearer "+token)

		if err != nil {
			fmt.Println("Error: ", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error making request:", err)
			return
		}
		defer resp.Body.Close()

		// Read response
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return
		}

		fmt.Println(string(body))

	}

}

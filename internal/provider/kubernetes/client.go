package kubernetes

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var ErrNotFound = errors.New("kubernetes resource not found")

type API interface {
	Create(context.Context, string, string, []byte) error
	Get(context.Context, string, string, string) ([]byte, error)
	List(context.Context, string, string, string) ([]byte, error)
	Logs(context.Context, string, string, string) ([]byte, error)
	Delete(context.Context, string, string, string) error
}

type ClientConfig struct {
	Endpoint              string
	Token                 string
	CAFile                string
	InsecureSkipTLSVerify bool
	HTTPClient            *http.Client
}

type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

func NewClient(config ClientConfig) (*Client, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("Kubernetes API server is required")
	}
	// Preserve the historical K8S_API_SERVER_HOST format (host[:port]).
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("invalid Kubernetes API server: %w", err)
	}
	if strings.TrimSpace(config.Token) == "" {
		return nil, fmt.Errorf("Kubernetes API bearer token is required")
	}
	client := config.HTTPClient
	if client == nil {
		transport, err := tlsTransport(config)
		if err != nil {
			return nil, err
		}
		client = &http.Client{Transport: transport}
	}
	return &Client{endpoint: endpoint, token: strings.TrimSpace(config.Token), http: client}, nil
}

func tlsTransport(config ClientConfig) (*http.Transport, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: config.InsecureSkipTLSVerify}
	if config.CAFile != "" {
		pem, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read Kubernetes CA: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("Kubernetes CA file contains no valid certificate")
		}
		tlsConfig.RootCAs = pool
	}
	return &http.Transport{TLSClientConfig: tlsConfig}, nil
}

func (c *Client) Create(ctx context.Context, namespace, resource string, body []byte) error {
	_, err := c.request(ctx, http.MethodPost, collectionPath(namespace, resource), body)
	return err
}

func (c *Client) Get(ctx context.Context, namespace, resource, name string) ([]byte, error) {
	return c.request(ctx, http.MethodGet, resourcePath(namespace, resource, name), nil)
}

func (c *Client) List(ctx context.Context, namespace, resource, labelSelector string) ([]byte, error) {
	path := collectionPath(namespace, resource)
	if labelSelector != "" {
		query := url.Values{"labelSelector": []string{labelSelector}}
		path += "?" + query.Encode()
	}
	return c.request(ctx, http.MethodGet, path, nil)
}

// ListCluster returns cluster-scoped Kubernetes resources such as nodes.
func (c *Client) ListCluster(ctx context.Context, resource string) ([]byte, error) {
	return c.request(ctx, http.MethodGet, "/api/v1/"+url.PathEscape(resource), nil)
}

func (c *Client) Logs(ctx context.Context, namespace, pod, container string) ([]byte, error) {
	query := url.Values{"container": []string{container}}
	path := resourcePath(namespace, "pods", pod) + "/log?" + query.Encode()
	return c.requestWithLimit(ctx, http.MethodGet, path, nil, 64<<20)
}

func (c *Client) Delete(ctx context.Context, namespace, resource, name string) error {
	_, err := c.request(ctx, http.MethodDelete, resourcePath(namespace, resource, name), []byte(`{"kind":"DeleteOptions","apiVersion":"v1","propagationPolicy":"Background"}`))
	return err
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return c.requestWithLimit(ctx, method, path, body, 1<<20)
}

func (c *Client) requestWithLimit(
	ctx context.Context,
	method string,
	path string,
	body []byte,
	limit int64,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Kubernetes API request: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, limit))
	if err != nil {
		return nil, fmt.Errorf("read Kubernetes API response: %w", err)
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Kubernetes API returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func collectionPath(namespace, resource string) string {
	base := "/api/v1"
	if resource == "jobs" {
		base = "/apis/batch/v1"
	}
	return fmt.Sprintf("%s/namespaces/%s/%s", base, url.PathEscape(namespace), resource)
}

func resourcePath(namespace, resource, name string) string {
	return collectionPath(namespace, resource) + "/" + url.PathEscape(name)
}

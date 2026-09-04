package gcp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const computeEndpoint = "https://compute.googleapis.com/compute/v1"

type Catalog struct {
	client          *http.Client
	computeEndpoint string
}

func New(client *http.Client) *Catalog {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &Catalog{client: client, computeEndpoint: computeEndpoint}
}

func (*Catalog) Provider() string { return "gcp" }

type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func (c *Catalog) Discover(ctx context.Context, connection domain.EnvironmentConnection, credential []byte) (domain.CloudCatalog, error) {
	var account serviceAccount
	if err := json.Unmarshal(credential, &account); err != nil {
		return domain.CloudCatalog{}, fmt.Errorf("decode GCP service account: %w", err)
	}
	project := configString(connection.Configuration, "projectId")
	if project == "" {
		project = account.ProjectID
	}
	if project == "" {
		return domain.CloudCatalog{}, fmt.Errorf("GCP project id is required")
	}
	token, err := c.accessToken(ctx, account)
	if err != nil {
		return domain.CloudCatalog{}, err
	}
	result := domain.CloudCatalog{Provider: "gcp", Project: project, Region: configString(connection.Configuration, "region"), Source: "live", DiscoveredAt: time.Now().UTC()}
	if err := c.machines(ctx, token, project, &result); err != nil {
		return domain.CloudCatalog{}, err
	}
	if err := c.disks(ctx, token, project, &result); err != nil {
		return domain.CloudCatalog{}, err
	}
	projects := []string{project, "ubuntu-os-cloud", "debian-cloud", "rocky-linux-cloud"}
	for _, imageProject := range projects {
		if err := c.images(ctx, token, project, imageProject, &result); err != nil {
			if imageProject == project {
				return domain.CloudCatalog{}, err
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("images from %s: %v", imageProject, err))
		}
	}
	sort.Slice(result.Machines, func(i, j int) bool { return result.Machines[i].ProviderTypeID < result.Machines[j].ProviderTypeID })
	sort.Slice(result.Images, func(i, j int) bool { return result.Images[i].Name < result.Images[j].Name })
	return result, nil
}

func (c *Catalog) accessToken(ctx context.Context, account serviceAccount) (string, error) {
	if account.ClientEmail == "" || account.PrivateKey == "" {
		return "", fmt.Errorf("GCP service account client_email and private_key are required")
	}
	tokenURI := account.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claim, _ := json.Marshal(map[string]any{"iss": account.ClientEmail, "scope": "https://www.googleapis.com/auth/compute.readonly", "aud": tokenURI, "iat": now, "exp": now + 3600})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claim)
	block, _ := pem.Decode([]byte(account.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("GCP private key is not PEM")
	}
	keyValue, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse GCP private key: %w", err)
	}
	key, ok := keyValue.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("GCP private key is not RSA")
	}
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	assertion := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request GCP access token: %w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("GCP authentication returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.AccessToken == "" {
		return "", fmt.Errorf("GCP authentication returned no access token")
	}
	return payload.AccessToken, nil
}

func (c *Catalog) get(ctx context.Context, token, endpoint string, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("GCP API returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode GCP response: %w", err)
	}
	return nil
}

func (c *Catalog) machines(ctx context.Context, token, project string, result *domain.CloudCatalog) error {
	var payload struct {
		Items map[string]struct {
			MachineTypes []struct {
				Name         string         `json:"name"`
				GuestCPUs    int            `json:"guestCpus"`
				MemoryMB     int64          `json:"memoryMb"`
				Zone         string         `json:"zone"`
				Architecture string         `json:"architecture"`
				Deprecated   map[string]any `json:"deprecated"`
				Accelerators []struct {
					Type  string `json:"guestAcceleratorType"`
					Count int    `json:"guestAcceleratorCount"`
				} `json:"accelerators"`
			} `json:"machineTypes"`
		} `json:"items"`
	}
	endpoint := fmt.Sprintf("%s/projects/%s/aggregated/machineTypes?returnPartialSuccess=true&maxResults=500", c.computeEndpoint, url.PathEscape(project))
	if err := c.get(ctx, token, endpoint, &payload); err != nil {
		return err
	}
	byType := map[string]int{}
	for _, scope := range payload.Items {
		for _, machine := range scope.MachineTypes {
			zone := lastPath(machine.Zone)
			region := zoneRegion(zone)
			if result.Region != "" && region != result.Region {
				continue
			}
			index, exists := byType[machine.Name]
			if !exists {
				gpuCount, gpuModel := 0, ""
				for _, gpu := range machine.Accelerators {
					gpuCount += gpu.Count
					if gpuModel == "" {
						gpuModel = gpu.Type
					}
				}
				architecture := strings.ToLower(machine.Architecture)
				if architecture == "" {
					architecture = "amd64"
				}
				value := domain.CloudMachineOffering{
					Provider:       "gcp",
					ProviderTypeID: machine.Name,
					Region:         region,
					Family:         strings.Split(machine.Name, "-")[0],
					VCPU:           machine.GuestCPUs,
					MemoryMiB:      machine.MemoryMB,
					Architecture:   architecture,
					GPUCount:       gpuCount,
					GPUModel:       gpuModel,
					SupportsSpot:   true,
					Available:      machine.Deprecated == nil,
				}
				result.Machines = append(result.Machines, value)
				index = len(result.Machines) - 1
				byType[machine.Name] = index
			}
			result.Machines[index].Zones = append(result.Machines[index].Zones, zone)
		}
	}
	return nil
}

func (c *Catalog) images(ctx context.Context, token, targetProject, imageProject string, result *domain.CloudCatalog) error {
	var payload struct {
		Items []struct {
			Name         string         `json:"name"`
			SelfLink     string         `json:"selfLink"`
			Family       string         `json:"family"`
			Architecture string         `json:"architecture"`
			Status       string         `json:"status"`
			DiskSizeGB   string         `json:"diskSizeGb"`
			Deprecated   map[string]any `json:"deprecated"`
		} `json:"items"`
	}
	endpoint := fmt.Sprintf("%s/projects/%s/global/images?filter=status%%3DREADY&maxResults=500", c.computeEndpoint, url.PathEscape(imageProject))
	if err := c.get(ctx, token, endpoint, &payload); err != nil {
		return err
	}
	for _, image := range payload.Items {
		source := "official"
		if imageProject == targetProject {
			source = "project-owned"
		}
		value := domain.CloudImageOffering{
			Provider:        "gcp",
			ProviderImageID: image.SelfLink,
			Name:            image.Name,
			Publisher:       imageProject,
			Family:          image.Family,
			OperatingSystem: imageOS(imageProject, image.Name),
			Architecture:    normalizeArchitecture(image.Architecture),
			Source:          source,
			Status:          strings.ToLower(image.Status),
			MinimumDiskGiB:  parseInt64(image.DiskSizeGB),
			Deprecated:      image.Deprecated != nil,
		}
		result.Images = append(result.Images, value)
	}
	return nil
}

func (c *Catalog) disks(ctx context.Context, token, project string, result *domain.CloudCatalog) error {
	var payload struct {
		Items map[string]struct {
			DiskTypes []struct {
				Name       string         `json:"name"`
				Zone       string         `json:"zone"`
				Deprecated map[string]any `json:"deprecated"`
			} `json:"diskTypes"`
		} `json:"items"`
	}
	endpoint := fmt.Sprintf("%s/projects/%s/aggregated/diskTypes?returnPartialSuccess=true&maxResults=500", c.computeEndpoint, url.PathEscape(project))
	if err := c.get(ctx, token, endpoint, &payload); err != nil {
		return err
	}
	byType := map[string]int{}
	for _, scope := range payload.Items {
		for _, disk := range scope.DiskTypes {
			zone := lastPath(disk.Zone)
			region := zoneRegion(zone)
			if result.Region != "" && region != result.Region {
				continue
			}
			index, exists := byType[disk.Name]
			if !exists {
				value := domain.CloudDiskOffering{Provider: "gcp", ProviderTypeID: disk.Name, Name: disk.Name, Region: region, Available: disk.Deprecated == nil}
				result.Disks = append(result.Disks, value)
				index = len(result.Disks) - 1
				byType[disk.Name] = index
			}
			result.Disks[index].Zones = append(result.Disks[index].Zones, zone)
		}
	}
	return nil
}

func configString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func lastPath(value string) string {
	parts := strings.Split(strings.TrimRight(value, "/"), "/")
	return parts[len(parts)-1]
}
func zoneRegion(zone string) string {
	parts := strings.Split(zone, "-")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "-")
}
func normalizeArchitecture(value string) string {
	value = strings.ToLower(value)
	if value == "x86_64" || value == "" {
		return "amd64"
	}
	return value
}
func imageOS(project, name string) string {
	value := strings.ToLower(project + " " + name)
	switch {
	case strings.Contains(value, "ubuntu"):
		return "ubuntu"
	case strings.Contains(value, "debian"):
		return "debian"
	case strings.Contains(value, "rocky"):
		return "rocky"
	default:
		return "linux"
	}
}
func parseInt64(value string) int64 {
	var result int64
	_, _ = fmt.Sscan(value, &result)
	return result
}

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
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const (
	computeEndpoint = "https://compute.googleapis.com/compute/v1"
	billingEndpoint = "https://cloudbilling.googleapis.com/v1/services/6F81-5844-456A/skus"
)

type Catalog struct {
	client          *http.Client
	computeEndpoint string
	billingEndpoint string
}

func New(client *http.Client) *Catalog {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &Catalog{client: client, computeEndpoint: computeEndpoint, billingEndpoint: billingEndpoint}
}

func (*Catalog) Provider() string { return "gcp" }

func (c *Catalog) ValidateCredential(ctx context.Context, connection domain.EnvironmentConnection, credential []byte) error {
	var account serviceAccount
	if err := json.Unmarshal(credential, &account); err != nil {
		return fmt.Errorf("decode GCP service account: %w", err)
	}
	_, err := c.accessToken(ctx, account)
	return err
}

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
	if err := c.prices(ctx, token, &result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("public pricing: %v", err))
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

type billingSKU struct {
	Description    string   `json:"description"`
	ServiceRegions []string `json:"serviceRegions"`
	Category       struct {
		ResourceFamily string `json:"resourceFamily"`
		ResourceGroup  string `json:"resourceGroup"`
		UsageType      string `json:"usageType"`
	} `json:"category"`
	PricingInfo []struct {
		PricingExpression struct {
			UsageUnit   string `json:"usageUnit"`
			TieredRates []struct {
				StartUsageAmount float64 `json:"startUsageAmount"`
				UnitPrice        struct {
					CurrencyCode string `json:"currencyCode"`
					Units        string `json:"units"`
					Nanos        int64  `json:"nanos"`
				} `json:"unitPrice"`
			} `json:"tieredRates"`
		} `json:"pricingExpression"`
	} `json:"pricingInfo"`
}

type familyPrice struct {
	core, memory float64
}

func (c *Catalog) prices(ctx context.Context, token string, result *domain.CloudCatalog) error {
	pageToken := ""
	skus := make([]billingSKU, 0, 5000)
	for {
		endpoint := c.billingEndpoint + "?currencyCode=USD&pageSize=5000"
		endpoint = pageEndpoint(endpoint, pageToken)
		var payload struct {
			SKUs          []billingSKU `json:"skus"`
			NextPageToken string       `json:"nextPageToken"`
		}
		if err := c.get(ctx, token, endpoint, &payload); err != nil {
			if strings.Contains(err.Error(), "cloudbilling.googleapis.com") && strings.Contains(err.Error(), "SERVICE_DISABLED") {
				return fmt.Errorf("Cloud Billing API is disabled for this credential project; enable cloudbilling.googleapis.com and refresh the catalog")
			}
			return err
		}
		skus = append(skus, payload.SKUs...)
		pageToken = payload.NextPageToken
		if pageToken == "" {
			break
		}
	}
	applyPrices(skus, result)
	return nil
}

func applyPrices(skus []billingSKU, result *domain.CloudCatalog) {
	families := map[string]*familyPrice{}
	for _, sku := range skus {
		if !skuInRegion(sku, result.Region) || !strings.EqualFold(sku.Category.UsageType, "OnDemand") {
			continue
		}
		price, currency := skuUnitPrice(sku)
		if price <= 0 || currency != "USD" {
			continue
		}
		description := strings.ToLower(sku.Description)
		group := strings.ToLower(sku.Category.ResourceGroup)
		for _, machine := range result.Machines {
			family := strings.ToLower(machine.Family)
			if family == "" || !strings.HasPrefix(description, family+" ") || strings.Contains(description, "sole tenancy") {
				continue
			}
			entry := families[family]
			if entry == nil {
				entry = &familyPrice{}
				families[family] = entry
			}
			if (group == "cpu" || group == "core") && strings.Contains(description, "core") {
				entry.core = price
			}
			if (group == "ram" || strings.Contains(group, "memory")) && strings.Contains(description, "ram") {
				entry.memory = price
			}
		}
	}
	for index := range result.Machines {
		machine := &result.Machines[index]
		price := families[strings.ToLower(machine.Family)]
		if price == nil || (price.core == 0 && price.memory == 0) {
			continue
		}
		machine.PricePerHour = price.core*float64(machine.VCPU) + price.memory*(float64(machine.MemoryMiB)/1024)
		machine.PricePerMinute = machine.PricePerHour / 60
		machine.PriceCurrency = "USD"
		machine.PriceSource = "gcp-public-catalog"
	}
	for index := range result.Disks {
		disk := &result.Disks[index]
		for _, sku := range skus {
			if !skuInRegion(sku, result.Region) || !strings.EqualFold(sku.Category.UsageType, "OnDemand") {
				continue
			}
			description := strings.ToLower(sku.Description)
			if !diskSKUDescription(disk.ProviderTypeID, description) {
				continue
			}
			price, currency := skuUnitPrice(sku)
			if price > 0 && currency == "USD" {
				disk.PricePerGiBMonth = price
				disk.PriceCurrency = currency
				disk.PriceSource = "gcp-public-catalog"
				break
			}
		}
	}
}

func skuInRegion(sku billingSKU, region string) bool {
	if region == "" || len(sku.ServiceRegions) == 0 {
		return true
	}
	for _, candidate := range sku.ServiceRegions {
		if candidate == region || candidate == "global" {
			return true
		}
	}
	return false
}

func skuUnitPrice(sku billingSKU) (float64, string) {
	if len(sku.PricingInfo) == 0 {
		return 0, ""
	}
	rates := sku.PricingInfo[len(sku.PricingInfo)-1].PricingExpression.TieredRates
	if len(rates) == 0 {
		return 0, ""
	}
	price := rates[0].UnitPrice
	units, _ := strconv.ParseFloat(price.Units, 64)
	return units + float64(price.Nanos)/1e9, price.CurrencyCode
}

func diskSKUDescription(diskType, description string) bool {
	if !strings.Contains(description, "capacity") || strings.Contains(description, "snapshot") {
		return false
	}
	switch diskType {
	case "pd-standard":
		return strings.Contains(description, "pd capacity") && !strings.Contains(description, "balanced") && !strings.Contains(description, "ssd")
	case "pd-balanced":
		return strings.Contains(description, "balanced") && strings.Contains(description, "capacity")
	case "pd-ssd":
		return strings.Contains(description, "ssd") && strings.Contains(description, "capacity")
	default:
		return strings.Contains(description, strings.ReplaceAll(diskType, "-", " "))
	}
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
	claim, _ := json.Marshal(map[string]any{"iss": account.ClientEmail, "scope": "https://www.googleapis.com/auth/cloud-platform", "aud": tokenURI, "iat": now, "exp": now + 3600})
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
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<20))
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
		NextPageToken string `json:"nextPageToken"`
		Items         map[string]struct {
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
	byType := map[string]int{}
	pageToken := ""
	for {
		endpoint := fmt.Sprintf("%s/projects/%s/aggregated/machineTypes?returnPartialSuccess=true&maxResults=500", c.computeEndpoint, url.PathEscape(project))
		endpoint = pageEndpoint(endpoint, pageToken)
		payload.Items, payload.NextPageToken = nil, ""
		if err := c.get(ctx, token, endpoint, &payload); err != nil {
			return err
		}
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
		pageToken = payload.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return nil
}

func (c *Catalog) images(ctx context.Context, token, targetProject, imageProject string, result *domain.CloudCatalog) error {
	var payload struct {
		NextPageToken string `json:"nextPageToken"`
		Items         []struct {
			Name         string         `json:"name"`
			SelfLink     string         `json:"selfLink"`
			Family       string         `json:"family"`
			Architecture string         `json:"architecture"`
			Status       string         `json:"status"`
			DiskSizeGB   string         `json:"diskSizeGb"`
			Deprecated   map[string]any `json:"deprecated"`
		} `json:"items"`
	}
	pageToken := ""
	for {
		endpoint := fmt.Sprintf("%s/projects/%s/global/images?filter=status%%3DREADY&maxResults=500", c.computeEndpoint, url.PathEscape(imageProject))
		endpoint = pageEndpoint(endpoint, pageToken)
		payload.Items, payload.NextPageToken = nil, ""
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
		pageToken = payload.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return nil
}

func (c *Catalog) disks(ctx context.Context, token, project string, result *domain.CloudCatalog) error {
	var payload struct {
		NextPageToken string `json:"nextPageToken"`
		Items         map[string]struct {
			DiskTypes []struct {
				Name       string         `json:"name"`
				Zone       string         `json:"zone"`
				Deprecated map[string]any `json:"deprecated"`
			} `json:"diskTypes"`
		} `json:"items"`
	}
	byType := map[string]int{}
	pageToken := ""
	for {
		endpoint := fmt.Sprintf("%s/projects/%s/aggregated/diskTypes?returnPartialSuccess=true&maxResults=500", c.computeEndpoint, url.PathEscape(project))
		endpoint = pageEndpoint(endpoint, pageToken)
		payload.Items, payload.NextPageToken = nil, ""
		if err := c.get(ctx, token, endpoint, &payload); err != nil {
			return err
		}
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
		pageToken = payload.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return nil
}

func pageEndpoint(endpoint, token string) string {
	if token == "" {
		return endpoint
	}
	return endpoint + "&pageToken=" + url.QueryEscape(token)
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

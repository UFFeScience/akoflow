// Package dockerhub queries public Docker Hub metadata; it never downloads images.
package dockerhub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Repository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Official    bool   `json:"official"`
}
type Image struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Digest       string `json:"digest"`
	Variant      string `json:"variant"`
	Size         int64  `json:"size"`
}
type Tag struct {
	Name   string  `json:"name"`
	Images []Image `json:"images"`
}
type Page[T any] struct {
	Results []T  `json:"results"`
	HasMore bool `json:"hasMore"`
}
type Selection struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}
type Error struct {
	Status     int
	RetryAfter string
	Message    string
}

func (e *Error) Error() string { return e.Message }

type cached struct {
	body    []byte
	expires time.Time
}
type Client struct {
	HTTP    *http.Client
	BaseURL string
	mu      sync.Mutex
	cache   map[string]cached
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, BaseURL: "https://hub.docker.com"}
}

var repositoryPattern = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*$`)
var tagPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`)
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func NormalizeRepository(name string) (string, error) {
	name = strings.TrimPrefix(strings.TrimSpace(name), "docker.io/")
	if !strings.Contains(name, "/") {
		name = "library/" + name
	}
	if len(name) > 255 || !repositoryPattern.MatchString(name) {
		return "", &Error{Status: 400, Message: "Enter a Docker Hub repository such as library/python or user/image"}
	}
	return name, nil
}
func (c *Client) get(ctx context.Context, path string, target any) error {
	c.mu.Lock()
	entry, ok := c.cache[path]
	c.mu.Unlock()
	if ok && time.Now().Before(entry.expires) {
		return json.Unmarshal(entry.body, target)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Akoflow/1.0")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return &Error{Status: 502, Message: "Could not reach Docker Hub. Try again."}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		status := 502
		message := "Docker Hub could not complete the request. Try again."
		if res.StatusCode == 404 {
			status = 404
			message = "Docker Hub repository or tag was not found."
		}
		if res.StatusCode == 429 {
			status = 429
			message = "Docker Hub request limit reached. Wait and retry."
		}
		return &Error{Status: status, RetryAfter: res.Header.Get("Retry-After"), Message: message}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, target); err != nil {
		return &Error{Status: 502, Message: "Docker Hub returned invalid metadata."}
	}
	c.mu.Lock()
	if c.cache == nil || len(c.cache) >= 128 {
		c.cache = map[string]cached{}
	}
	c.cache[path] = cached{body, time.Now().Add(time.Minute)}
	c.mu.Unlock()
	return nil
}
func (c *Client) Search(ctx context.Context, query string, page int) (Page[Repository], error) {
	result := Page[Repository]{Results: []Repository{}}
	query = strings.TrimSpace(query)
	if len(query) < 2 || len(query) > 100 {
		return result, &Error{Status: 400, Message: "Search must contain 2 to 100 characters."}
	}
	var response struct {
		Next    string `json:"next"`
		Results []struct {
			Name        string `json:"repo_name"`
			Description string `json:"short_description"`
			Official    bool   `json:"is_official"`
		} `json:"results"`
	}
	err := c.get(ctx, fmt.Sprintf("/v2/search/repositories/?query=%s&page_size=20&page=%d", url.QueryEscape(query), page), &response)
	for _, item := range response.Results {
		name, e := NormalizeRepository(item.Name)
		if e == nil {
			result.Results = append(result.Results, Repository{name, item.Description, item.Official})
		}
	}
	result.HasMore = response.Next != ""
	return result, err
}
func (c *Client) Tags(ctx context.Context, repository, query string, page int) (Page[Tag], error) {
	result := Page[Tag]{Results: []Tag{}}
	name, err := NormalizeRepository(repository)
	if err != nil {
		return result, err
	}
	if len(query) > 128 {
		return result, &Error{Status: 400, Message: "Tag search is too long."}
	}
	var response struct {
		Next    *string `json:"next"`
		Results []Tag   `json:"results"`
	}
	err = c.get(ctx, fmt.Sprintf("/v2/namespaces/%s/repositories/%s/tags?page_size=20&page=%d&name=%s", strings.Split(name, "/")[0], strings.Split(name, "/")[1], page, url.QueryEscape(query)), &response)
	for _, tag := range response.Results {
		images := []Image{}
		for _, image := range tag.Images {
			if image.OS == "linux" && digestPattern.MatchString(image.Digest) {
				images = append(images, image)
			}
		}
		tag.Images = images
		if len(images) > 0 {
			result.Results = append(result.Results, tag)
		}
	}
	result.HasMore = response.Next != nil && *response.Next != ""
	return result, err
}
func (c *Client) Resolve(ctx context.Context, selection Selection, architecture string) (string, error) {
	name, err := NormalizeRepository(selection.Repository)
	if err != nil {
		return "", err
	}
	if !tagPattern.MatchString(selection.Tag) {
		return "", &Error{Status: 400, Message: "Select a valid Docker Hub tag."}
	}
	var tag Tag
	parts := strings.Split(name, "/")
	if err = c.get(ctx, fmt.Sprintf("/v2/namespaces/%s/repositories/%s/tags/%s", parts[0], parts[1], url.PathEscape(selection.Tag)), &tag); err != nil {
		return "", err
	}
	matches := []Image{}
	for _, image := range tag.Images {
		if image.OS == "linux" && image.Architecture == architecture && digestPattern.MatchString(image.Digest) {
			matches = append(matches, image)
		}
	}
	if len(matches) != 1 {
		return "", &Error{Status: 422, Message: "Select a tag with one unambiguous Linux image for the chosen architecture."}
	}
	return "docker.io/" + name + "@" + matches[0].Digest, nil
}

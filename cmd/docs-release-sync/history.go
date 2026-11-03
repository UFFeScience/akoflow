package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const githubReleases = "https://api.github.com/repos/UFFeScience/akoflow/releases"
const canonicalReleasePath = "/repositories/827382806/releases"

var nextPage = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

type asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type release struct {
	ID          int64   `json:"id"`
	Tag         string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	CreatedAt   string  `json:"created_at"`
	Draft       bool    `json:"draft,omitempty"`
	Prerelease  bool    `json:"prerelease"`
	Assets      []asset `json:"assets"`
}

func fetchHistory(ctx context.Context, client *http.Client, endpoint, token string) ([]release, error) {
	first, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := first.Query()
	query.Set("per_page", "100")
	query.Set("page", "1")
	first.RawQuery = query.Encode()
	next := first.String()
	visited := make(map[string]bool)
	history := make([]release, 0)
	for next != "" {
		target, err := url.Parse(next)
		if err != nil {
			return nil, err
		}
		validPath := target.Path == first.Path || target.Path == canonicalReleasePath
		if target.Scheme != first.Scheme || target.Host != first.Host || !validPath || visited[next] {
			return nil, fmt.Errorf("unexpected GitHub release pagination link")
		}
		visited[next] = true
		page, link, err := fetchPage(ctx, client, target.String(), token)
		if err != nil {
			return nil, err
		}
		for _, item := range page {
			if !item.Draft && strings.TrimSpace(item.Tag) != "" {
				history = append(history, item)
			}
		}
		next = ""
		if match := nextPage.FindStringSubmatch(link); len(match) > 1 {
			next = match[1]
		}
	}
	return history, nil
}

func fetchPage(ctx context.Context, client *http.Client, endpoint, token string) ([]release, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GitHub release request returned HTTP %d", response.StatusCode)
	}
	var page []release
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&page); err != nil {
		return nil, "", fmt.Errorf("decode GitHub releases: %w", err)
	}
	if page == nil {
		return nil, "", fmt.Errorf("GitHub releases response is not a list")
	}
	return page, response.Header.Get("Link"), nil
}

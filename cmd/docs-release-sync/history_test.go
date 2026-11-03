package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSynchronizeAllPagesExcludesDraftsAndSecrets(t *testing.T) {
	var server *httptest.Server
	var calls atomic.Int32
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing server-side authentication")
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<`+server.URL+canonicalReleasePath+`?per_page=100&page=2>; rel="next"`)
			json.NewEncoder(w).Encode([]release{{ID: 1, Tag: "v1", Body: "First"}, {ID: 2, Tag: "draft", Draft: true}})
			return
		}
		if r.URL.Path != canonicalReleasePath {
			t.Errorf("unexpected pagination route %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]release{{ID: 3, Tag: "v2-beta", Prerelease: true, Body: "Edited notes", Assets: []asset{{Name: "Desktop.zip", Size: 123}}}})
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "releases.json")
	if err := synchronize(context.Background(), server.Client(), server.URL+"/releases", "test-secret", output, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var result snapshot
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || len(result.Releases) != 2 || !result.Releases[1].Prerelease || result.Releases[1].Body != "Edited notes" {
		t.Fatalf("incomplete public history: calls=%d releases=%+v", calls.Load(), result.Releases)
	}
	if bytes.Contains(data, []byte("test-secret")) {
		t.Fatal("credential leaked into the browser snapshot")
	}
}

func TestUnavailableGitHubKeepsThePreviousSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "history.json")
	if err := writeSnapshot(path, snapshot{UpdatedAt: "2026-11-02", Releases: []release{{Tag: "v1"}}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	var warnings bytes.Buffer
	if err := synchronize(context.Background(), server.Client(), server.URL+"/releases", "", path, &warnings); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || !strings.Contains(warnings.String(), "retaining") {
		t.Fatal("fallback history was overwritten")
	}
	if err := synchronize(context.Background(), server.Client(), server.URL+"/releases", "", path+".missing", io.Discard); err == nil {
		t.Fatal("first synchronization failure without a valid snapshot was hidden")
	}
}

func TestPaginationRejectsAnotherOriginBeforeSendingCredentials(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Link", `<https://example.com/releases>; rel="next"`)
		w.Write([]byte("[]"))
	}))
	defer server.Close()
	if _, err := fetchHistory(context.Background(), server.Client(), server.URL+"/releases", "secret"); err == nil || calls.Load() != 1 {
		t.Fatalf("unsafe pagination was accepted: calls=%d err=%v", calls.Load(), err)
	}
}

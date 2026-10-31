package dockerhub

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := New()
	client.BaseURL = server.URL
	return client
}
func TestSearchNormalizesOfficialImagesAndCachesMetadata(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v2/search/repositories/" || r.URL.Query().Get("query") != "busy box" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"next":"more","results":[{"repo_name":"busybox","is_official":true,"short_description":"Small"},{"repo_name":"user/tool"}]}`))
	})
	for i := 0; i < 2; i++ {
		page, err := client.Search(context.Background(), "busy box", 1)
		if err != nil || len(page.Results) != 2 || page.Results[0].Name != "library/busybox" || !page.Results[0].Official || !page.HasMore {
			t.Fatalf("page=%+v error=%v", page, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected cached response, calls=%d", calls.Load())
	}
}
func TestTagsFilterUnsupportedImagesAndUsePagination(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/namespaces/library/repositories/python/tags" || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("name") != "3.12" {
			t.Errorf("URL %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"next":null,"results":[{"name":"3.12","images":[{"os":"linux","architecture":"amd64","digest":"sha256:` + strings.Repeat("a", 64) + `"},{"os":"unknown","architecture":"unknown"}]},{"name":"windows","images":[{"os":"windows","architecture":"amd64"}]}]}`))
	})
	page, err := client.Tags(context.Background(), "python", "3.12", 2)
	if err != nil || len(page.Results) != 1 || len(page.Results[0].Images) != 1 || page.HasMore {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
func TestResolvePinsLinuxPlatformDigestAndRejectsAmbiguousPlatforms(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ambiguous") {
			_, _ = w.Write([]byte(`{"images":[{"os":"linux","architecture":"arm","digest":"sha256:` + strings.Repeat("a", 64) + `"},{"os":"linux","architecture":"arm","digest":"sha256:` + strings.Repeat("b", 64) + `"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"images":[{"os":"linux","architecture":"amd64","digest":"sha256:` + strings.Repeat("a", 64) + `"}]}`))
	})
	ref, err := client.Resolve(context.Background(), Selection{"python", "3.12"}, "amd64")
	if err != nil || ref != "docker.io/library/python@sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("ref=%s err=%v", ref, err)
	}
	for _, input := range []struct{ tag, arch string }{{"3.12", "arm64"}, {"ambiguous", "arm"}, {"../bad", "amd64"}} {
		if _, err = client.Resolve(context.Background(), Selection{"python", input.tag}, input.arch); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
}
func TestUpstreamRateLimitsAndBadResponses(t *testing.T) {
	for _, status := range []int{429, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(status)
			})
			_, err := client.Search(context.Background(), "python", 1)
			var upstream *Error
			if !errors.As(err, &upstream) {
				t.Fatalf("error=%v", err)
			}
			expected := status
			if expected == 500 {
				expected = 502
			}
			if upstream.Status != expected {
				t.Fatalf("status=%d", upstream.Status)
			}
			if status == 429 && upstream.RetryAfter != "30" {
				t.Fatal("missing retry delay")
			}
		})
	}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not JSON`)) })
	if _, err := client.Search(context.Background(), "python", 1); err == nil {
		t.Fatal("accepted invalid JSON")
	}
}
func TestInvalidRepositoryDoesNotMakeNetworkRequests(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected upstream request") })
	for _, name := range []string{"https://evil.example/x", "../bad", "user/image/extra", "name?x=y", "user/%2e%2e"} {
		if _, err := client.Tags(context.Background(), name, "", 1); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}

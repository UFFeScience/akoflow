package mcpbridge

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header = r.Header.Clone()
	copy.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(copy)
}

func TestStreamableHTTPAuthenticationAndMCPClient(t *testing.T) {
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"env-one"}]`))
	})
	handler, err := bridge.HTTPHandler(HTTPOptions{Token: "mcp-secret", AllowedOrigins: []string{"https://trusted.example"}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	for _, test := range []struct {
		token, origin string
		status        int
	}{
		{"", "", 401}, {"wrong", "", 401}, {"mcp-secret", "https://hostile.example", 403},
	} {
		request := httptest.NewRequest("POST", "/mcp", strings.NewReader("{}"))
		request.Header.Set("Authorization", "Bearer "+test.token)
		request.Header.Set("Origin", test.origin)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("status=%d want=%d", recorder.Code, test.status)
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{"mcp-secret", http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result := callTool(t, session, "list_environments", map[string]any{})
	if result.IsError {
		t.Fatalf("HTTP call failed: %#v", result)
	}
	data := resultMap(t, result)["data"].([]any)
	if data[0].(map[string]any)["id"] != "env-one" {
		t.Fatal("wrong HTTP MCP response")
	}
	if _, err = bridge.HTTPHandler(HTTPOptions{}); err == nil {
		t.Fatal("unauthenticated HTTP allowed")
	}
}

func TestOAuthDiscoveryAndTokenValidation(t *testing.T) {
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	issuer := ""
	resource := "http://127.0.0.1:8123/mcp"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, secret, ok := r.BasicAuth()
		if !ok || client != "introspector" || secret != "provider-secret" {
			t.Error("missing introspection authentication")
		}
		r.ParseForm()
		result := map[string]any{
			"active": true, "iss": issuer, "aud": resource, "sub": "user-one",
			"scope": "akoflow:read", "exp": time.Now().Add(time.Hour).Unix(),
		}
		switch r.Form.Get("token") {
		case "inactive":
			result["active"] = false
		case "wrong-audience":
			result["aud"] = "another-resource"
		case "wrong-issuer":
			result["iss"] = "another-issuer"
		case "expired":
			result["exp"] = time.Now().Add(-time.Hour).Unix()
		case "wrong-scope":
			result["scope"] = "unrelated"
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer provider.Close()
	issuer = provider.URL
	handler, err := bridge.HTTPHandler(HTTPOptions{OAuth: &OAuthOptions{
		Issuer: issuer, Resource: resource, IntrospectionURL: provider.URL + "/introspect",
		ClientID: "introspector", ClientSecret: "provider-secret",
	}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	discovery := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource/mcp", nil)
	discovery.Host = "127.0.0.1:8123"
	handler.ServeHTTP(recorder, discovery)
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), issuer) {
		t.Fatal("OAuth resource discovery missing")
	}
	for _, token := range []string{"inactive", "wrong-audience", "wrong-issuer", "expired", "wrong-scope", ""} {
		request := httptest.NewRequest("POST", "/mcp", strings.NewReader("{}"))
		request.Host = "127.0.0.1:8123"
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 401 && response.Code != 403 {
			t.Fatalf("%s status=%d", token, response.Code)
		}
		if response.Header().Get("WWW-Authenticate") == "" {
			t.Fatal("missing OAuth challenge")
		}
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "oauth-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{"valid", http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if callTool(t, session, "health_check", map[string]any{}).IsError {
		t.Fatal("valid OAuth token rejected")
	}
}

func TestLargeResponsesRemainRetrievableAndRedacted(t *testing.T) {
	payload := strings.Repeat("x", 2<<20)
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"payload": payload, "token": "never-return-this", "credentialRef": "safe-ref"})
	})
	session := testSession(t, bridge)
	result := callTool(t, session, "list_environments", map[string]any{})
	if result.IsError {
		t.Fatal("large response failed")
	}
	id := resultMap(t, result)["downloadId"].(string)
	file, err := bridge.transfers.open(id)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "never-return-this") {
		t.Fatal("staged download leaked secret")
	}
	var decoded map[string]any
	if json.Unmarshal(data, &decoded) != nil {
		t.Fatal("staged response is invalid JSON")
	}
	if decoded["payload"] != payload || decoded["credentialRef"] != "safe-ref" {
		t.Fatal("response was truncated or corrupted")
	}
	result = callTool(t, session, "read_transfer", map[string]any{"downloadId": id, "offset": 0, "length": maxChunk})
	if result.IsError {
		t.Fatal("chunk read failed")
	}
}

func TestUploadsAreImmutableBoundedAndMultipart(t *testing.T) {
	bridge := testBridge(t, Write, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		file, _, err := r.FormFile("context")
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "archive-bytes" {
			t.Errorf("multipart bytes=%q", data)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"uploaded"}`))
	})
	session := testSession(t, bridge)
	begun := callTool(t, session, "begin_upload", map[string]any{})
	id := resultMap(t, begun)["uploadId"].(string)
	chunk := map[string]any{"uploadId": id, "offset": 0, "data": "YXJjaGl2ZS1ieXRlcw=="}
	for range 2 {
		if callTool(t, session, "upload_chunk", chunk).IsError {
			t.Fatal("identical chunk retry rejected")
		}
	}
	if !callTool(t, session, "seal_upload", map[string]any{"uploadId": id, "size": 999}).IsError {
		t.Fatal("incorrect seal size accepted")
	}
	if callTool(t, session, "seal_upload", map[string]any{"uploadId": id, "size": 13}).IsError {
		t.Fatal("seal failed")
	}
	if !callTool(t, session, "upload_chunk", chunk).IsError {
		t.Fatal("sealed upload changed")
	}
	result := callTool(t, session, "save_build_context", map[string]any{"uploadId": id, "requestId": "build-upload"})
	if result.IsError {
		t.Fatalf("multipart tool failed: %#v", result)
	}
	if _, _, err := bridge.transfers.read(id, 0, maxChunk+1); err == nil {
		t.Fatal("oversized read accepted")
	}
	bridge.transfers.mu.Lock()
	bridge.transfers.items[id].expires = time.Now().Add(-time.Second)
	bridge.transfers.mu.Unlock()
	if _, err := bridge.transfers.open(id); err == nil {
		t.Fatal("expired upload remained accessible")
	}
}

func TestCredentialUploadIsNotDownloadableAndErrorsAreRedacted(t *testing.T) {
	const secret = "credential-upload-secret-value"
	bridge := testBridge(t, Admin, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("credential upload media type")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(map[string]any{"error": "rejected " + secret})
	})
	session := testSession(t, bridge)
	id, _, err := bridge.transfers.save(strings.NewReader(`{"id":"kube","token":"credential-upload-secret-value"}`))
	if err != nil {
		t.Fatal(err)
	}
	download := callTool(t, session, "read_transfer", map[string]any{"downloadId": id, "offset": 0, "length": 512})
	if !download.IsError {
		t.Fatal("credential upload was exposed as a download")
	}
	op := findOperation(t, "save_kubernetes_token")
	args := map[string]any{"uploadId": id, "requestId": "credential-upload"}
	confirmed(t, session, op, args)
	result := callTool(t, session, op.Name, args)
	raw, _ := json.Marshal(result)
	if !result.IsError || strings.Contains(string(raw), secret) {
		t.Fatalf("credential error leaked: %s", raw)
	}
}

func TestLegacyHTTPProtocolVersions(t *testing.T) {
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	handler, err := bridge.HTTPHandler(HTTPOptions{Token: "legacy-token"})
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"2025-03-26", "2025-06-18", "2025-11-25"} {
		t.Run(version, func(t *testing.T) {
			params := map[string]any{
				"protocolVersion": version, "capabilities": map[string]any{},
				"clientInfo": map[string]any{"name": "legacy-client", "version": "1"},
			}
			for _, method := range []string{"initialize", "tools/list"} {
				payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
				if method == "initialize" {
					payload["params"] = params
				}
				body, _ := json.Marshal(payload)
				request := httptest.NewRequest("POST", "/mcp", strings.NewReader(string(body)))
				request.Header.Set("Authorization", "Bearer legacy-token")
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Accept", "application/json, text/event-stream")
				request.Header.Set("MCP-Protocol-Version", version)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != 200 {
					t.Fatalf("%s status=%d body=%s", method, response.Code, response.Body)
				}
				var value map[string]any
				if json.Unmarshal(response.Body.Bytes(), &value) != nil || value["error"] != nil {
					t.Fatalf("%s failed: %s", method, response.Body)
				}
			}
		})
	}
}

func TestConfiguredPublicHostWorksBehindLoopbackProxy(t *testing.T) {
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	handler, err := bridge.HTTPHandler(HTTPOptions{Token: "proxy-token", PublicURL: "https://mcp.example.com/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"proxy-client","version":"1"}}}`
	for _, test := range []struct {
		host   string
		status int
	}{
		{"mcp.example.com", 200}, {"127.0.0.1:8080", 200}, {"untrusted.example", 403},
	} {
		request := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
		request.Host = test.host
		local := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		request = request.WithContext(context.WithValue(request.Context(), http.LocalAddrContextKey, local))
		request.Header.Set("Authorization", "Bearer proxy-token")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("host=%s status=%d body=%s", test.host, response.Code, response.Body)
		}
	}
}

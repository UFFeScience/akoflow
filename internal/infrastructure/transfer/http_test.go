package transfer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestHTTPDownloadChecksAndReadsRanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		if request.Header.Get("Range") != "bytes=2-" {
			t.Errorf("range=%q", request.Header.Get("Range"))
		}
		_, _ = response.Write([]byte("payload"))
	}))
	defer server.Close()
	connector := HTTPDownload{Client: server.Client()}
	endpoint := domain.TransferEndpoint{URI: server.URL + "/artifact"}
	if !connector.CanHandle(endpoint) {
		t.Fatal("HTTP endpoint was not recognized")
	}
	exists, err := connector.Exists(context.Background(), endpoint, "")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	reader, err := connector.Open(context.Background(), endpoint, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(payload) != "payload" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	if err := connector.Put(context.Background(), endpoint, "", nil, 0); err == nil {
		t.Fatal("HTTP destination must fail")
	}
	if err := connector.Commit(context.Background(), endpoint, "a", "b"); err == nil {
		t.Fatal("HTTP commit must fail")
	}
}

func TestHTTPDownloadReportsMissingObject(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	connector := HTTPDownload{Client: server.Client()}
	endpoint := domain.TransferEndpoint{URI: server.URL}
	exists, err := connector.Exists(context.Background(), endpoint, "")
	if err != nil || exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	if _, err := connector.Open(context.Background(), endpoint, "", 0); err == nil {
		t.Fatal("open must report HTTP failure")
	}
}

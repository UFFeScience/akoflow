package s3

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDriverUsesObjectHTTPContract(t *testing.T) {
	content := []byte("result")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(request.Body)
			if !bytes.Equal(body, content) {
				t.Errorf("body=%q", body)
			}
			response.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			_, _ = response.Write(content)
		case http.MethodHead:
			response.Header().Set("Content-Length", "6")
			response.Header().Set("ETag", `"checksum"`)
		case http.MethodDelete:
			response.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	driver := New(server.Client(), nil)
	location, err := driver.Put(context.Background(), ports.PutObjectRequest{
		Storage: domain.StorageResource{ID: "s3", Type: domain.StorageS3, Endpoint: server.URL},
		Key:     "run/activity/result.txt", Source: bytes.NewReader(content), Size: int64(len(content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := driver.Get(context.Background(), ports.GetObjectRequest{Location: location, Target: &output}); err != nil {
		t.Fatal(err)
	}
	stat, err := driver.Stat(context.Background(), location)
	if err != nil || output.String() != "result" || stat.SizeBytes != 6 {
		t.Fatalf("output=%q stat=%+v err=%v", output.String(), stat, err)
	}
	if err := driver.Delete(context.Background(), location); err != nil {
		t.Fatal(err)
	}
}

func TestBrowseUsesPrefixAndDelimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("delimiter") != "/" || r.URL.Query().Get("prefix") != "datasets/" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(
			`<ListBucketResult>` +
				`<CommonPrefixes><Prefix>datasets/raw/</Prefix></CommonPrefixes>` +
				`<Contents><Key>datasets/input.csv</Key><Size>7</Size><ETag>etag</ETag></Contents>` +
				`<IsTruncated>false</IsTruncated>` +
				`</ListBucketResult>`,
		))
	}))
	defer server.Close()
	d := New(server.Client(), nil)
	page, err := d.Browse(context.Background(), domain.StorageResource{ID: "s3", Endpoint: server.URL}, domain.BrowseRequest{Path: "datasets"})
	if err != nil || len(page.Entries) != 2 || page.Entries[0].Type != domain.FileEntryDirectory {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type credentialStub struct {
	err  error
	refs []string
}

func (c *credentialStub) Authorize(_ context.Context, request *http.Request, reference string) error {
	c.refs = append(c.refs, reference)
	request.Header.Set("Authorization", "test")
	return c.err
}

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

func TestBrowseOpenWriteRemoveAndStatEntry(t *testing.T) {
	content := []byte("payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "test" {
			t.Error("request was not authorized")
		}
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Content-Length", "7")
			w.Header().Set("ETag", `"etag"`)
		case http.MethodGet:
			if r.URL.Query().Get("list-type") == "2" {
				_, _ = w.Write([]byte(`<ListBucketResult><Contents><Key>root/</Key><Size>0</Size></Contents><Contents><Key>root/file.txt</Key><Size>7</Size><ETag>"etag"</ETag></Contents><NextContinuationToken>next</NextContinuationToken><IsTruncated>true</IsTruncated></ListBucketResult>`))
				return
			}
			_, _ = w.Write(content)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			if !bytes.Equal(body, content) {
				t.Errorf("body=%q", body)
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	credentials := &credentialStub{}
	driver := New(server.Client(), credentials)
	storage := domain.StorageResource{ID: "s3", Endpoint: server.URL + "/{key}", CredentialReference: "secret"}
	page, err := driver.Browse(context.Background(), storage, domain.BrowseRequest{Path: "root", Cursor: "cursor"})
	if err != nil || len(page.Entries) != 1 || page.NextCursor != "next" || page.Entries[0].ETag != "etag" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	entry, err := driver.BrowseStat(context.Background(), storage, "folder/file.txt")
	if err != nil || entry.SizeBytes != 7 || entry.ETag != "etag" || entry.Name != "file.txt" {
		t.Fatalf("entry=%+v err=%v", entry, err)
	}
	reader, opened, err := driver.Open(context.Background(), storage, "folder/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	openedContent, _ := io.ReadAll(reader)
	_ = reader.Close()
	if !bytes.Equal(openedContent, content) || opened.SizeBytes != 7 {
		t.Fatalf("content=%q entry=%+v", openedContent, opened)
	}
	if err := driver.Write(context.Background(), storage, "folder/file.txt", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if err := driver.Remove(context.Background(), storage, "folder/file.txt"); err != nil {
		t.Fatal(err)
	}
	storage.ReadOnly = true
	if err := driver.Remove(context.Background(), storage, "folder/file.txt"); err == nil {
		t.Fatal("read-only storage removal must fail")
	}
	if len(credentials.refs) < 5 {
		t.Fatalf("credential calls=%d", len(credentials.refs))
	}
}

func TestDriverReportsHTTPAndCredentialFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	storage := domain.StorageResource{Endpoint: server.URL, CredentialReference: "secret"}
	driver := New(server.Client(), nil)
	if _, err := driver.Browse(context.Background(), storage, domain.BrowseRequest{}); err == nil {
		t.Fatal("browse HTTP error expected")
	}
	if _, err := driver.StatEntry(context.Background(), storage, "key"); err == nil {
		t.Fatal("stat HTTP error expected")
	}
	if _, _, err := driver.Open(context.Background(), storage, "key"); err == nil {
		t.Fatal("open stat error expected")
	}
	if err := driver.Delete(context.Background(), domain.DataLocation{URI: server.URL + "/key"}); err == nil {
		t.Fatal("delete HTTP error expected")
	}
	credentials := &credentialStub{err: errors.New("denied")}
	if _, err := New(server.Client(), credentials).Stat(context.Background(), domain.DataLocation{URI: server.URL, Metadata: map[string]any{"credentialReference": "secret"}}); err == nil {
		t.Fatal("credential error expected")
	}
}

func TestS3PathValidationAndMetadata(t *testing.T) {
	if New(nil, nil).Type() != domain.StorageS3 {
		t.Fatal("unexpected storage type")
	}
	if _, err := objectPrefix("../outside"); err == nil {
		t.Fatal("escaping prefix must fail")
	}
	if prefix, err := objectPrefix(" /"); err != nil || prefix != "" {
		t.Fatalf("prefix=%q err=%v", prefix, err)
	}
	for _, item := range []struct{ endpoint, key string }{{"", "key"}, {"ftp://host", "key"}, {"https://host", ""}, {"https://host", "../key"}} {
		if _, err := objectURL(item.endpoint, item.key); err == nil {
			t.Fatalf("invalid object URL accepted: %+v", item)
		}
	}
	value, err := objectURL("https://host/bucket/{key}", "folder/a b.txt")
	if err != nil || value != "https://host/bucket/folder/a%20b.txt" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	if metadataString(map[string]any{"key": "value"}, "key") != "value" || metadataString(nil, "key") != "" {
		t.Fatal("metadata string conversion failed")
	}
}

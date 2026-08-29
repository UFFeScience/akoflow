package transfer

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type s3CredentialResolverStub struct {
	values S3Credentials
	err    error
	ref    string
}

func (s *s3CredentialResolverStub) Resolve(ref string) (S3Credentials, error) {
	s.ref = ref
	return s.values, s.err
}

func s3Endpoint(server *httptest.Server) domain.TransferEndpoint {
	return domain.TransferEndpoint{URI: "s3://bucket/prefix", Configuration: map[string]string{
		"endpoint": strings.TrimPrefix(server.URL, "http://"), "secure": "false", "credentialRef": "test",
	}}
}

func TestEnvironmentS3Credentials(t *testing.T) {
	resolver := EnvironmentS3Credentials{}
	if _, err := resolver.Resolve("unknown"); err == nil {
		t.Fatal("unknown credential reference must fail")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	if _, err := resolver.Resolve("env"); err == nil {
		t.Fatal("missing environment credentials must fail")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_SESSION_TOKEN", "token")
	credentials, err := resolver.Resolve("")
	if err != nil || credentials.AccessKey != "key" || credentials.SecretKey != "secret" || credentials.SessionToken != "token" {
		t.Fatalf("credentials=%+v err=%v", credentials, err)
	}
}

func TestS3EndpointAndObjectValidation(t *testing.T) {
	connector := S3Compatible{Credentials: &s3CredentialResolverStub{values: S3Credentials{AccessKey: "key", SecretKey: "secret"}}}
	if !connector.CanHandle(domain.TransferEndpoint{URI: "s3://bucket"}) || connector.CanHandle(domain.TransferEndpoint{URI: "https://bucket"}) {
		t.Fatal("unexpected endpoint handling")
	}
	for _, endpoint := range []domain.TransferEndpoint{{URI: "://invalid"}, {URI: "https://bucket"}, {URI: "s3:///prefix"}} {
		if _, _, _, err := connector.s3Client(endpoint); err == nil {
			t.Fatalf("invalid endpoint accepted: %+v", endpoint)
		}
	}
	if s3Object("prefix", "folder/file") != "prefix/folder/file" || s3Object("prefix", "../escape") != "" || s3Object("", "") != "" {
		t.Fatal("unexpected object normalization")
	}
	if _, err := connector.Exists(context.Background(), domain.TransferEndpoint{URI: "https://bucket"}, "file"); err == nil {
		t.Fatal("invalid endpoint must fail before stat")
	}
}

func TestS3ExistsOpenPutAndCommit(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		if strings.Contains(r.URL.Path, "missing") {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`))
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Has("location"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint></LocationConstraint>`))
		case r.Method == http.MethodHead:
			w.Header().Set("ETag", `"etag"`)
			w.Header().Set("Content-Length", "7")
			w.Header().Set("Last-Modified", "Sat, 29 Aug 2026 00:00:00 GMT")
		case r.Method == http.MethodGet:
			w.Header().Set("Last-Modified", "Sat, 29 Aug 2026 00:00:00 GMT")
			w.Header().Set("ETag", `"etag"`)
			if r.Header.Get("Range") == "bytes=2-" {
				w.Header().Set("Content-Length", "5")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("yload"))
				return
			}
			w.Header().Set("Content-Length", "7")
			_, _ = w.Write([]byte("payload"))
		case r.Method == http.MethodPost && r.URL.Query().Has("uploads"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>prefix/file</Key><UploadId>upload</UploadId></InitiateMultipartUploadResult>`))
		case r.Method == http.MethodPut && r.URL.Query().Get("partNumber") != "":
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("ETag", `"part"`)
		case r.Method == http.MethodPost && r.URL.Query().Get("uploadId") != "":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<CompleteMultipartUploadResult><Location>test</Location><Bucket>bucket</Bucket><Key>prefix/file</Key><ETag>"etag"</ETag></CompleteMultipartUploadResult>`))
		case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"etag"</ETag><LastModified>2026-08-29T00:00:00Z</LastModified></CopyObjectResult>`))
		case r.Method == http.MethodPut:
			_, _ = io.Copy(io.Discard, r.Body)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	resolver := &s3CredentialResolverStub{values: S3Credentials{AccessKey: "key", SecretKey: "secret"}}
	connector := S3Compatible{Credentials: resolver, BufferSize: func(context.Context) int { return 5 << 20 }}
	endpoint := s3Endpoint(server)
	exists, err := connector.Exists(context.Background(), endpoint, "file")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	exists, err = connector.Exists(context.Background(), endpoint, "missing")
	if err != nil || exists {
		t.Fatalf("missing exists=%v err=%v", exists, err)
	}
	reader, err := connector.Open(context.Background(), endpoint, "file", 2)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(content) != "yload" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	if err := connector.Put(context.Background(), endpoint, "file", bytes.NewReader([]byte("new")), 0); err != nil {
		t.Fatal(err)
	}
	if err := connector.Commit(context.Background(), endpoint, "partial", "final"); err != nil {
		t.Fatal(err)
	}
	if resolver.ref != "test" || len(requests) < 6 {
		t.Fatalf("credential ref=%q requests=%+v", resolver.ref, requests)
	}
}

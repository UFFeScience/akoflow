package server_connector_workflow

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCreateSendsWorkflow(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s", request.Method)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content type = %s", request.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(request.Body)
		if string(body) != `{"workflow":"encoded"}` {
			t.Fatalf("body = %s", body)
		}
		return response(http.StatusCreated, `{"workflow":"encoded","message":"created"}`), nil
	})}

	if err := NewWithClient(client).Create("example.test", "8080", "encoded"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateReportsTransportFailure(t *testing.T) {
	expected := errors.New("offline")
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, expected
	})}
	if err := NewWithClient(client).Create("example.test", "8080", "encoded"); !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateRejectsHTTPFailure(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusBadRequest, `{}`), nil
	})}
	if err := NewWithClient(client).Create("example.test", "8080", "encoded"); err == nil {
		t.Fatal("expected status error")
	}
}

func TestCreateRejectsInvalidResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{`), nil
	})}
	if err := NewWithClient(client).Create("example.test", "8080", "encoded"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestNewWithClientUsesDefaultForNil(t *testing.T) {
	if NewWithClient(nil).client != http.DefaultClient {
		t.Fatal("expected default client")
	}
}

func TestNewCreatesDedicatedClient(t *testing.T) {
	if New().client == nil {
		t.Fatal("expected HTTP client")
	}
}

func TestCreateRejectsInvalidURL(t *testing.T) {
	if err := New().Create("bad\nhost", "8080", "encoded"); err == nil {
		t.Fatal("expected request creation error")
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

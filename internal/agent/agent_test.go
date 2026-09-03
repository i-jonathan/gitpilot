package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGenerate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		json.NewEncoder(w).Encode(generateResponse{
			Response: "```\nfeat(cli): add user authentication\n```",
		})
	}))
	defer srv.Close()

	a := Agent{
		Model:   "test-model",
		BaseURL: srv.URL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}

	msg, err := a.Generate("some diff")
	if err != nil {
		t.Fatal(err)
	}
	if msg != "```\nfeat(cli): add user authentication\n```" {
		t.Errorf("got %q, want raw model output", msg)
	}
}

func TestGenerate_ModelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": `model "qwen3" not found`,
		})
	}))
	defer srv.Close()

	a := Agent{
		Model:   "qwen3",
		BaseURL: srv.URL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}

	_, err := a.Generate("diff")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Model not found") {
		t.Errorf("got %q, want 'Model not found'", err)
	}
}

func TestGenerate_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "internal failure",
		})
	}))
	defer srv.Close()

	a := Agent{
		Model:   "test",
		BaseURL: srv.URL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}

	_, err := a.Generate("diff")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("got %q, want 'HTTP 500'", err)
	}
}

func TestGenerate_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	a := Agent{
		Model:   "test",
		BaseURL: srv.URL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}

	_, err := a.Generate("diff")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "malformed response from Ollama") {
		t.Errorf("got %q, want 'malformed response'", err)
	}
}

func TestGenerate_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(generateResponse{Response: ""})
	}))
	defer srv.Close()

	a := Agent{
		Model:   "test",
		BaseURL: srv.URL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}

	msg, err := a.Generate("diff")
	if err != nil {
		t.Fatal(err)
	}
	if msg != "" {
		t.Errorf("got %q, want empty string", msg)
	}
}

func TestGenerate_ConnectionError(t *testing.T) {
	a := Agent{
		Model:   "test",
		BaseURL: "http://127.0.0.1:1",
		Client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	_, err := a.Generate("diff")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Could not connect to Ollama") {
		t.Errorf("got %q, want 'Could not connect to Ollama'", err)
	}
}

func TestFriendlyNetError_ConnectionRefused(t *testing.T) {
	baseURL := "http://localhost:11434"
	netErr := &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")}
	urlErr := &url.Error{Op: "Post", URL: baseURL + "/api/generate", Err: netErr}

	result := friendlyNetError(urlErr, baseURL)
	msg := result.Error()
	if !strings.Contains(msg, "Could not connect to Ollama") {
		t.Errorf("got %q, want connection-refused message", msg)
	}
	if !strings.Contains(msg, baseURL) {
		t.Errorf("got %q, want baseURL in message", msg)
	}
}

func TestFriendlyNetError_DNSFailure(t *testing.T) {
	baseURL := "http://localhost:11434"
	dnsErr := &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("no such host")}
	urlErr := &url.Error{Op: "Post", URL: baseURL + "/api/generate", Err: dnsErr}

	result := friendlyNetError(urlErr, baseURL)
	msg := result.Error()
	if !strings.Contains(msg, "Could not connect to Ollama") {
		t.Errorf("got %q, want connection-refused message", msg)
	}
}

func TestFriendlyNetError_Timeout(t *testing.T) {
	baseURL := "http://localhost:11434"
	timeoutErr := &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("i/o timeout")}
	urlErr := &url.Error{Op: "Post", URL: baseURL + "/api/generate", Err: timeoutErr}

	result := friendlyNetError(urlErr, baseURL)
	msg := result.Error()
	if !strings.Contains(msg, "Could not connect to Ollama") {
		t.Errorf("got %q, want connection-refused message", msg)
	}
}

func TestFriendlyNetError_NonNetError(t *testing.T) {
	ctxErr := fmt.Errorf("context canceled")
	urlErr := &url.Error{Op: "Post", URL: "http://localhost:11434/api/generate", Err: ctxErr}

	result := friendlyNetError(urlErr, "http://localhost:11434")
	msg := result.Error()
	if !strings.Contains(msg, "context canceled") {
		t.Errorf("got %q, want original error message", msg)
	}
}

func TestFriendlyNetError_NonURLError(t *testing.T) {
	other := fmt.Errorf("something else entirely")
	result := friendlyNetError(other, "http://localhost:11434")
	if !strings.Contains(result.Error(), "something else entirely") {
		t.Errorf("friendlyNetError changed non-network error: %v", result)
	}
}

func TestFriendlyNetError_URLErrorNilInner(t *testing.T) {
	urlErr := &url.Error{Op: "Post", URL: "http://localhost:11434/api/generate", Err: nil}
	result := friendlyNetError(urlErr, "http://localhost:11434")
	if result == nil {
		t.Fatal("expected non-nil error")
	}
}

func TestFriendlyHTTPError_ModelNotFound(t *testing.T) {
	body := `{"error":"model \"qwen3\" not found, try pulling it first"}`
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := friendlyHTTPError(resp)
	if !strings.Contains(err.Error(), "Model not found") {
		t.Errorf("got %q, want 'Model not found'", err)
	}
	if !strings.Contains(err.Error(), "qwen3") {
		t.Errorf("got %q, want model name in message", err)
	}
}

func TestFriendlyHTTPError_InternalServerErrorWithMessage(t *testing.T) {
	body := `{"error":"internal server failure"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := friendlyHTTPError(resp)
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 500") {
		t.Errorf("got %q, want 'HTTP 500'", msg)
	}
	if !strings.Contains(msg, "internal server failure") {
		t.Errorf("got %q, want error message from body", msg)
	}
}

func TestFriendlyHTTPError_PlainBody(t *testing.T) {
	body := "Bad Gateway"
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := friendlyHTTPError(resp)
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 502") {
		t.Errorf("got %q, want 'HTTP 502'", msg)
	}
}

func TestFriendlyHTTPError_NotFoundWithoutErrorField(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("404 page not found")),
	}

	err := friendlyHTTPError(resp)
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 404") {
		t.Errorf("got %q, want 'HTTP 404'", msg)
	}
}
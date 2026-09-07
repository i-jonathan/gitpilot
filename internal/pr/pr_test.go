package pr

import (
	"encoding/json"
	"errors"
	"gitpilot/internal/agent"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalize_Success(t *testing.T) {
	pr := PullRequest{
		Title:       "  feat: add new feature  ",
		Description: "  This is a description  ",
	}
	if err := pr.normalize(); err != nil {
		t.Fatal(err)
	}
	if pr.Title != "feat: add new feature" {
		t.Errorf("Title = %q, want %q", pr.Title, "feat: add new feature")
	}
	if pr.Description != "This is a description" {
		t.Errorf("Description = %q, want %q", pr.Description, "This is a description")
	}
}

func TestNormalize_EmptyTitle(t *testing.T) {
	pr := PullRequest{
		Title:       "  ",
		Description: "some description",
	}
	err := pr.normalize()
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if !strings.Contains(err.Error(), "title is empty") {
		t.Errorf("got %q, want 'title is empty'", err)
	}
}

func TestNormalize_EmptyDescription(t *testing.T) {
	pr := PullRequest{
		Title:       "feat: add feature",
		Description: "  \n  ",
	}
	err := pr.normalize()
	if err == nil {
		t.Fatal("expected error for empty description")
	}
	if !strings.Contains(err.Error(), "description is empty") {
		t.Errorf("got %q, want 'description is empty'", err)
	}
}

func TestNormalize_TitleTooLong(t *testing.T) {
	title := strings.Repeat("a", 101)
	pr := PullRequest{
		Title:       title,
		Description: "some description",
	}
	err := pr.normalize()
	if err == nil {
		t.Fatal("expected error for title exceeding 100 characters")
	}
	if !strings.Contains(err.Error(), "exceeds 100 characters") {
		t.Errorf("got %q, want 'exceeds 100 characters'", err)
	}
}

func TestNormalize_TitleExactly100(t *testing.T) {
	title := strings.Repeat("a", 100)
	pr := PullRequest{
		Title:       title,
		Description: "valid description",
	}
	if err := pr.normalize(); err != nil {
		t.Fatalf("expected no error for exactly 100 chars, got: %v", err)
	}
}

func TestParseError_Error(t *testing.T) {
	inner := errors.New("invalid character")
	pe := &ParseError{
		Response: `{"title": broken}`,
		Err:      inner,
	}
	msg := pe.Error()
	if !strings.Contains(msg, "parse pull request response") {
		t.Errorf("Error() = %q, want 'parse pull request response'", msg)
	}
	if !strings.Contains(msg, inner.Error()) {
		t.Errorf("Error() = %q, want inner error", msg)
	}
}

func TestParseError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	pe := &ParseError{
		Response: "bad response",
		Err:      inner,
	}
	if !errors.Is(pe, inner) {
		t.Error("errors.Is should find the wrapped error")
	}
	if unwrapped := pe.Unwrap(); unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}
}

func TestBuildPrompt(t *testing.T) {
	commits := "feat: add login\nfix: fix bug"
	diff := "diff --git a/main.go b/main.go\n+func main() {}"
	prompt := buildPrompt(commits, diff)

	if !strings.Contains(prompt, commits) {
		t.Error("buildPrompt should include the commits")
	}
	if !strings.Contains(prompt, diff) {
		t.Error("buildPrompt should include the diff")
	}
	if !strings.Contains(prompt, "COMMITS") {
		t.Error("buildPrompt should include COMMITS section")
	}
	if !strings.Contains(prompt, "CODE CHANGES") {
		t.Error("buildPrompt should include CODE CHANGES section")
	}
	if !strings.Contains(prompt, "OUTPUT FORMAT") {
		t.Error("buildPrompt should mention OUTPUT FORMAT")
	}
	if !strings.Contains(prompt, "STRICT RULES") {
		t.Error("buildPrompt should mention STRICT RULES")
	}
}

func TestGenerate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		resp := map[string]string{
			"response": `{"title":"feat: add login","description":"Add login functionality"}`,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	result, err := generate(a, "some prompt")
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "feat: add login" {
		t.Errorf("Title = %q, want %q", result.Title, "feat: add login")
	}
	if result.Description != "Add login functionality" {
		t.Errorf("Description = %q, want %q", result.Description, "Add login functionality")
	}
}

func TestGenerate_ParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"response": "not valid json at all",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	_, err := generate(a, "some prompt")
	if err == nil {
		t.Fatal("expected error for bad JSON response")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *ParseError, got %T", err)
	}
	if parseErr.Response != "not valid json at all" {
		t.Errorf("Response = %q, want %q", parseErr.Response, "not valid json at all")
	}
}

func TestGenerate_TrimsWhitespace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"response": `  {"title":"feat: login","description":"desc"}  `,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	result, err := generate(a, "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "feat: login" {
		t.Errorf("Title = %q, want %q", result.Title, "feat: login")
	}
}

func TestGenerate_NetworkError(t *testing.T) {
	a := createTestAgent("http://127.0.0.1:1")
	_, err := generate(a, "prompt")
	if err == nil {
		t.Fatal("expected network error")
	}
	if !strings.Contains(err.Error(), "generate pull request error") {
		t.Errorf("got %q, want 'generate pull request error'", err)
	}
}

func TestGenerateValid_SuccessOnFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"response": `{"title":"feat: add login","description":"Add login"}`,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	pr, err := generateValid(a, "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if pr.Title != "feat: add login" {
		t.Errorf("Title = %q, want %q", pr.Title, "feat: add login")
	}
}

func TestGenerateValid_RetryOnParseError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			resp := map[string]string{
				"response": "not json",
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp := map[string]string{
			"response": `{"title":"feat: add login","description":"Add login"}`,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	pr, err := generateValid(a, "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if pr.Title != "feat: add login" {
		t.Errorf("Title = %q, want %q", pr.Title, "feat: add login")
	}
}

func TestGenerateValid_RetryOnNormalizationError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// Empty title will fail normalize()
			resp := map[string]string{
				"response": `{"title":"  ","description":"desc"}`,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp := map[string]string{
			"response": `{"title":"feat: add login","description":"Add login"}`,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	pr, err := generateValid(a, "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if pr.Title != "feat: add login" {
		t.Errorf("Title = %q, want %q", pr.Title, "feat: add login")
	}
}

func TestGenerateValid_FailAfterTwoParseErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"response": "not json",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	_, err := generateValid(a, "prompt")
	if err == nil {
		t.Fatal("expected error after two failed attempts")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *ParseError, got %T", err)
	}
}

func TestGenerateValid_FailAfterTwoNormalizationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"response": `{"title":"","description":"desc"}`,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := createTestAgent(srv.URL)
	_, err := generateValid(a, "prompt")
	if err == nil {
		t.Fatal("expected error after two failed normalization attempts")
	}
	if !strings.Contains(err.Error(), "title is empty") {
		t.Errorf("got %q, want 'title is empty'", err)
	}
}

func TestGenerateValid_NonParseErrorStopsImmediately(t *testing.T) {
	a := createTestAgent("http://127.0.0.1:1")
	_, err := generateValid(a, "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "generate pull request error") {
		t.Errorf("got %q, want 'generate pull request error'", err)
	}
}

// Helper to create a test agent pointing at a given server URL
func createTestAgent(baseURL string) *agent.Agent {
	return &agent.Agent{
		Model:   "test-model",
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}
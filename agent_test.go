package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feat(cli): add logging", "feat(cli): add logging"},
		{"  feat: add logging  ", "feat: add logging"},
		{`"feat: add logging"`, "feat: add logging"},
		{"'feat: add logging'", "feat: add logging"},
		{"```\nfeat: add logging\n```", "feat: add logging"},
		{"`feat: add logging`", "feat: add logging"},
		{"feat: add logging\n\nmore text here", "feat: add logging"},
		{"```feat: add logging```", "feat: add logging"},
		{"\"```feat: add logging```\"", "feat: add logging"},
	}

	for _, tt := range tests {
		got, err := normalizeMessage(tt.input)
		if err != nil {
			t.Errorf("normalizeMessage(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeMessage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeMessage_Empty(t *testing.T) {
	tests := []string{"", "  ", "\n", "```\n```", `""`, "''"}

	for _, input := range tests {
		_, err := normalizeMessage(input)
		if !errors.Is(err, errEmptyResponse) {
			t.Errorf("normalizeMessage(%q) = %v, want errEmptyResponse", input, err)
		}
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feat: add logging", ""},
		{"feat(cli): add logging", ""},
		{"fix(api): handle edge case", ""},
		{"docs(readme): update install", ""},
		{"feat!: breaking change", ""},
		{"feat(api)!: breaking change", ""},
		{"", "commit message is empty"},
		{"a-commit-message-without-valid-format", "does not follow"},
		{"no-colon-here", "does not follow"},
	}

	for _, tt := range tests {
		err := validateMessage(tt.input)
		if tt.want == "" {
			if err != nil {
				t.Errorf("validateMessage(%q) = %v, want nil", tt.input, err)
			}
		} else {
			if err == nil {
				t.Errorf("validateMessage(%q) = nil, want error containing %q", tt.input, tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("validateMessage(%q) = %v, want error containing %q", tt.input, err, tt.want)
			}
		}
	}
}

func TestValidateMessage_Over72(t *testing.T) {
	long := "feat: " + strings.Repeat("a", 67) // 73 chars total
	err := validateMessage(long)
	if err == nil {
		t.Fatal("expected error for 73-char message")
	}
	if !strings.Contains(err.Error(), "72") {
		t.Errorf("got %v, want error mentioning 72", err)
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

func TestFriendlyNetError_OtherError(t *testing.T) {
	other := fmt.Errorf("something else")
	result := friendlyNetError(other, "http://localhost:11434")
	if !strings.Contains(result.Error(), "something else") {
		t.Errorf("friendlyNetError changed non-network error: %v", result)
	}
}

func TestCommitMessageFlow(t *testing.T) {
	msg, err := normalizeMessage("```\nfeat(cli): add user authentication\n```")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMessage(msg); err != nil {
		t.Fatal(err)
	}
	if msg != "feat(cli): add user authentication" {
		t.Errorf("got %q, want %q", msg, "feat(cli): add user authentication")
	}
}
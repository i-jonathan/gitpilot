package cli

import (
	"strings"
	"testing"
)

func TestRequireCommand_Success(t *testing.T) {
	if err := RequireCommand("git"); err != nil {
		t.Errorf("RequireCommand(\"git\") = %v, want nil", err)
	}
}

func TestRequireCommand_Failure(t *testing.T) {
	err := RequireCommand("nonexistent-command-xyz-123")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("got %q, want 'not found in PATH'", err)
	}
}

func TestRequireGithubAuth_Error(t *testing.T) {
	err := RequireGithubAuth()
	if err != nil {
		// Either gh is not installed or not authenticated — both are valid
		if !strings.Contains(err.Error(), "GitHub") {
			t.Errorf("got %q, want message mentioning GitHub", err)
		}
	}
	// If gh is available and authenticated, this test passes without error
}
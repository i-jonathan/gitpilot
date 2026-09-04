package commit

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
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
		{"`'feat: add logging'`", "feat: add logging"},
		{"'`feat: add logging`'", "feat: add logging"},
		{"\nfeat: add logging", "feat: add logging"},
	}

	for _, tt := range tests {
		got, err := normalize(tt.input)
		if err != nil {
			t.Errorf("normalize(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalize_Empty(t *testing.T) {
	tests := []string{"", "  ", "\n", "```\n```", `""`, "''", "`\n`", "\"\"\n", "```\n```\n"}

	for _, input := range tests {
		_, err := normalize(input)
		if !errors.Is(err, errEmptyResponse) {
			t.Errorf("normalize(%q) = %v, want errEmptyResponse", input, err)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feat: add logging", ""},
		{"feat(cli): add logging", ""},
		{"fix(api): handle edge case", ""},
		{"feat!: breaking change", ""},
		{"feat(api)!: breaking change", ""},
		{"chore: bump version", ""},
		{"refactor(core): extract helper", ""},
		{"feat: " + strings.Repeat("a", 65), ""},
		{"", "commit message is empty"},
		{"a-commit-message-without-valid-format", "does not follow"},
		{"no-colon-here", "does not follow"},
		{":starts-with-colon", "does not follow"},
		{"feat : spaced", "does not follow"},
		{"feat(api : space", "does not follow"},
	}

	for _, tt := range tests {
		err := validate(tt.input)
		if tt.want == "" {
			if err != nil {
				t.Errorf("validate(%q) = %v, want nil", tt.input, err)
			}
		} else {
			if err == nil {
				t.Errorf("validate(%q) = nil, want error containing %q", tt.input, tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("validate(%q) = %v, want error containing %q", tt.input, err, tt.want)
			}
		}
	}
}

func TestValidate_Over72(t *testing.T) {
	long := "feat: " + strings.Repeat("a", 67)
	err := validate(long)
	if err == nil {
		t.Fatal("expected error for 73-char message")
	}
	if !strings.Contains(err.Error(), "72") {
		t.Errorf("got %v, want error mentioning 72", err)
	}
}

func TestIsValidCCPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"feat", true},
		{"fix", true},
		{"chore", true},
		{"docs", true},
		{"feat(api)", true},
		{"fix(core)", true},
		{"", false},
		{"feat api", false},
		{"feat (api)", false},
		{"feat(", false},
		{"feat)", false},
		{"(api)", false},
		{"feat()", false},
	}

	for _, tt := range tests {
		got := isValidCCPrefix(tt.input)
		if got != tt.want {
			t.Errorf("isValidCCPrefix(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBuildPrompt(t *testing.T) {
	diff := "diff --git a/main.go b/main.go\nindex abc..def\n+// new comment"
	prompt := buildPrompt(diff)

	if !strings.Contains(prompt, diff) {
		t.Errorf("buildPrompt should include the diff in the prompt")
	}
	if !strings.Contains(prompt, "Conventional Commit") {
		t.Errorf("buildPrompt should mention Conventional Commits")
	}
}

func TestNormalizeAndValidateFlow(t *testing.T) {
	msg, err := normalize("```\nfeat(cli): add user authentication\n```")
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(msg); err != nil {
		t.Fatal(err)
	}
	if msg != "feat(cli): add user authentication" {
		t.Errorf("got %q, want %q", msg, "feat(cli): add user authentication")
	}
}

func TestNormalizeAndValidateFlow_FullSanitization(t *testing.T) {
	msg, err := normalize("`feat(core): improve performance`")
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(msg); err != nil {
		t.Fatal(err)
	}
	if msg != "feat(core): improve performance" {
		t.Errorf("got %q, want %q", msg, "feat(core): improve performance")
	}
}
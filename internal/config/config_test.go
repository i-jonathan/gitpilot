package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultsWhenFileMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "qwen3.5:4b" {
		t.Errorf("default model = %q, want qwen3.5:4b", c.Model)
	}
	if c.APIURL != "http://localhost:11434" {
		t.Errorf("default api_url = %q, want http://localhost:11434", c.APIURL)
	}
}

func TestLoad_ReadsExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "gitpilot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"model":"llama3","api_url":"http://10.0.0.1:11434","thinking":true}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "llama3" {
		t.Errorf("model = %q, want llama3", c.Model)
	}
	if c.APIURL != "http://10.0.0.1:11434" {
		t.Errorf("api_url = %q, want http://10.0.0.1:11434", c.APIURL)
	}
	if !c.Thinking {
		t.Error("thinking = false, want true")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "gitpilot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`not-json`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Errorf("got %v, want 'parse config' error", err)
	}
}

func TestLoad_MinimalJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "gitpilot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "" {
		t.Errorf("model = %q, want empty", c.Model)
	}
}

func TestLoad_ReadError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "gitpilot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.json")
	if err := os.RemoveAll(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(configPath, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when config is a directory")
	}
}
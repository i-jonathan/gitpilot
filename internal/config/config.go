package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Model    string `json:"model"`
	APIURL   string `json:"api_url"`
	Thinking bool   `json:"thinking"`
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	path := filepath.Join(home, ".config", "gitpilot", "config.json")
	data, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		return Config{
			Model:    "qwen3.5:4b",
			APIURL:   "http://localhost:11434",
			Thinking: false,
		}, nil
	}

	if err != nil {
		return Config{}, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return c, nil
}
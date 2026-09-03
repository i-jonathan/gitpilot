package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Think  bool   `json:"think"`
}

type generateResponse struct {
	Response string `json:"response"`
}

type Agent struct {
	BaseURL string
	Model   string
	Think   bool
	Client  *http.Client
}

func (a *Agent) CommitMessage(prompt string) (string, error) {
	reqBody := generateRequest{
		Model:  a.Model,
		Prompt: prompt,
		Stream: false,
		Think:  a.Think,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost, a.BaseURL+"/api/generate", bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("agent request: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Println(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("agent returned HTTP code %d", resp.StatusCode)
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode agent response: %w", err)
	}

	return strings.TrimSpace(result.Response), nil
}

func buildCommitPrompt(diff string) string {
	return fmt.Sprintf(`You are an expert software engineer generating Git commit messages.

	Analyze the staged Git diff below and generate a concise Conventional Commit message.

	Rules:
	- Use Conventional Commits format.
	- Format: type(scope): description
	- Keep the subject under 72 characters.
	- Use imperative mood.
	- Describe the purpose of the change, not implementation details.
	- Do not invent information.
	- Do not include markdown.
	- Do not include a body.
	- Return only the commit message.

	Staged diff:

	%s`, diff)
}

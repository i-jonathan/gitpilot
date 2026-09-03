package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
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

var errEmptyResponse = errors.New("generated commit message is empty")

func (a *Agent) CommitMessage(prompt string) (string, error) {
	reqBody := generateRequest{
		Model:  a.Model,
		Prompt: prompt,
		Stream: false,
		Think:  a.Think,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, a.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", friendlyNetError(err, a.BaseURL)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Println(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", friendlyHTTPError(resp)
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("malformed response from Ollama: %w", err)
	}

	msg, err := normalizeMessage(result.Response)
	if err != nil {
		return "", err
	}

	if err := validateMessage(msg); err != nil {
		return "", fmt.Errorf("generated message is invalid: %w", err)
	}

	return msg, nil
}

func normalizeMessage(msg string) (string, error) {
	msg = strings.TrimSpace(msg)

	// Remove surrounding quotes
	if len(msg) >= 2 && msg[0] == '"' && msg[len(msg)-1] == '"' {
		msg = msg[1 : len(msg)-1]
	}
	if len(msg) >= 2 && msg[0] == '\'' && msg[len(msg)-1] == '\'' {
		msg = msg[1 : len(msg)-1]
	}
	msg = strings.TrimSpace(msg)

	// Remove markdown code fences
	msg = strings.TrimPrefix(msg, "```")
	msg = strings.TrimSuffix(msg, "```")
	msg = strings.TrimSpace(msg)
	msg = strings.TrimPrefix(msg, "`")
	msg = strings.TrimSuffix(msg, "`")
	msg = strings.TrimSpace(msg)

	// Take only the first line (single subject)
	if idx := strings.Index(msg, "\n"); idx != -1 {
		msg = strings.TrimSpace(msg[:idx])
	}

	if msg == "" {
		return "", errEmptyResponse
	}

	return msg, nil
}

func validateMessage(msg string) error {
	if msg == "" {
		return errors.New("commit message is empty")
	}
	if len(msg) > 72 {
		return fmt.Errorf("commit message exceeds 72 characters (%d)", len(msg))
	}

	col := strings.Index(msg, ":")
	if col < 1 {
		return errors.New("commit message does not follow Conventional Commits format")
	}

	prefix := strings.TrimSuffix(msg[:col], "!")
	if !isValidConventionalCommitPrefix(prefix) {
		return errors.New("commit message does not follow Conventional Commits format")
	}
	return nil
}

func isValidConventionalCommitPrefix(s string) bool {
	if !strings.Contains(s, "(") && !strings.Contains(s, ")") {
		return s != "" && !strings.ContainsAny(s, " \t")
	}
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 1 || close != len(s)-1 || close <= open {
		return false
	}
	return !strings.ContainsAny(s[:open], " \t")
}

func friendlyNetError(err error, baseURL string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		if _, ok := errors.AsType[*net.OpError](urlErr.Err); ok {
			return fmt.Errorf(
				"Could not connect to Ollama at %s.\n\n"+
					"Make sure Ollama is running and try again.",
				baseURL,
			)
		}
	}
	return err
}

func friendlyHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(body, &errResp)

	if resp.StatusCode == http.StatusNotFound && errResp.Error != "" {
		return fmt.Errorf("Model not found: %s", errResp.Error)
	}
	if errResp.Error != "" {
		return fmt.Errorf("Ollama returned HTTP %d: %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("Ollama returned HTTP %d", resp.StatusCode)
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

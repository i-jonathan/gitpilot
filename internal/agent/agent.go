package agent

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

var ErrEmptyResponse = errors.New("generated commit message is empty")

func (a *Agent) Generate(prompt string) (string, error) {
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

	return strings.TrimSpace(result.Response), nil
}

func friendlyNetError(err error, baseURL string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		// var netErr *net.OpError
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

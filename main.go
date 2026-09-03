package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	Model    string `json:"model"`
	APIURL   string `json:"api_url"`
	Thinking bool   `json:"thinking"`
}

func main() {
	command := parseCommand(os.Args)

	switch command {
	case "commit":
		c, err := loadConfig()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		commitCommand(c)
	default:
		usage()
	}

}

func parseCommand(args []string) string {
	if len(args) < 2 {
		usage()
		os.Exit(1)
	}

	return args[1]
}

func usage() {
	fmt.Println(`Usage: gitpilot commit`)
}

func loadConfig() (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, err
	}

	path := filepath.Join(home, ".config", "gitpilot", "config.json")
	data, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		return config{
			Model:    "qwen3.5:4b",
			APIURL:   "http://localhost:11434",
			Thinking: false,
		}, nil
	}

	if err != nil {
		return config{}, err
	}

	var c config
	if err := json.Unmarshal(data, &c); err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}

	return c, nil
}

func promptAction() (string, error) {
	fmt.Println("\n[a]ccept [r]etry [c]ancel")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.ToLower(strings.TrimSpace(input)), nil
}

func commitCommand(c config) {
	err := gitAvailable()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rootDir, err := gitRoot()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	hasStaged, err := hasStagedChanges(rootDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if !hasStaged {
		fmt.Println("Feature is yet to be implemented. Please stage the files you want to commit")
		return
	}

	diff, err := getStagedDiff(rootDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	prompt := buildCommitPrompt(diff)

	agent := Agent{
		Model:   c.Model,
		BaseURL: c.APIURL,
		Think:   c.Thinking,
		Client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}

	commitMessage, err := agent.CommitMessage(prompt)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for {
		fmt.Println("\nSuggested commit message:")
		fmt.Printf("    %s\n", commitMessage)

		action, err := promptAction()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		switch action {
		case "a", "accept":
			err = commit(rootDir, commitMessage)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			os.Exit(0)
		case "r", "retry":
			commitMessage, err = agent.CommitMessage(prompt)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		case "c", "cancel":
			fmt.Println("Cancelled.")
			os.Exit(0)
		default:
			fmt.Println("Please choose a, r, or c.")
		}
	}
}

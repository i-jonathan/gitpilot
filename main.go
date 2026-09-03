package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type config struct {
	Model    string `json:"model"`
	APIURL   string `json:"api_url"`
	Thinking bool   `json:"thinking"`
}

var ErrCancelled = errors.New("cancelled")
var ErrNoChanges = errors.New("no file changes")

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

func promptStagingAction() (string, error) {
	fmt.Println("\nNo staged changes.")
	fmt.Println("\nWhat files would you like to stage?")
	fmt.Println("\n[a]dd all [s]elect files [c]ancel")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	switch strings.ToLower(strings.TrimSpace(input)) {
	case "a", "all", "add":
		return "all", nil
	case "s", "select":
		return "select", nil
	case "c", "cancel":
		return "cancel", nil
	default:
		return "", fmt.Errorf("invalid option: %q", strings.TrimSpace(input))
	}
}

func promptFileSelection(files []gitFile) ([]string, error) {
	fmt.Println("\nChanged files:")

	for i, file := range files {
		fmt.Printf("    [%d] %s   %s\n", i+1, file.Status, file.Path)
	}

	fmt.Println("\n Select files (e.g. 1,3,4) or [c]ancel: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	input = strings.TrimSpace(input)

	if strings.EqualFold(input, "c") || strings.EqualFold(input, "cancel") {
		return nil, nil
	}

	parts := strings.Split(input, ",")
	selectedFiles := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		index, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid file selection: %q", part)
		}

		if index < 1 || index > len(files) {
			return nil, fmt.Errorf("file selection %d is out of range", index)
		}

		selectedFiles = append(selectedFiles, files[index-1].Path)
	}

	return selectedFiles, nil
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
		err := handleUnstagedChanges(rootDir)
		if errors.Is(err, ErrCancelled) {
			fmt.Println("Cancelled.")
			return
		}

		if errors.Is(err, ErrNoChanges) {
			fmt.Println("No file changes to commit.")
			return
		}

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		hasStaged, err = hasStagedChanges(rootDir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if !hasStaged {
			fmt.Println("staging files failed")
			os.Exit(1)
		}
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

func handleUnstagedChanges(rootDir string) error {
	files, err := changedFiles(rootDir)
	if err != nil {
		return err
	}

	if len(files) < 1 {
		return ErrNoChanges
	}

	action, err := promptStagingAction()
	if err != nil {
		return err
	}

	switch action {
	case "all":
		err = stageAll(rootDir)
		if err != nil {
			return err
		}
	case "select":
		selection, err := promptFileSelection(files)
		if err != nil {
			return err
		}

		if len(selection) == 0 {
			return ErrCancelled
		}

		err = stageFiles(rootDir, selection)
		if err != nil {
			return err
		}
	case "cancel":
		return ErrCancelled
	default:
		return fmt.Errorf("invalid staging action: %q", action)
	}

	return nil
}

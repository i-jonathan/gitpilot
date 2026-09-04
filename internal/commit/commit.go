package commit

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gitpilot/internal/agent"
	"gitpilot/internal/config"
	"gitpilot/internal/git"
)

var ErrCancelled = errors.New("cancelled")
var ErrNoChanges = errors.New("no file changes")
var errEmptyResponse = errors.New("generated commit message is empty")

func Run(cfg config.Config) error {
	if err := git.Available(); err != nil {
		return err
	}

	rootDir, err := git.Root()
	if err != nil {
		return err
	}

	repo := git.NewRepo(rootDir)

	hasStaged, err := repo.HasStagedChanges()
	if err != nil {
		return err
	}

	if !hasStaged {
		err := handleUnstagedChanges(repo)
		if errors.Is(err, ErrCancelled) {
			fmt.Println("Cancelled.")
			return nil
		}
		if errors.Is(err, ErrNoChanges) {
			fmt.Println("No file changes to commit.")
			return nil
		}
		if err != nil {
			return err
		}

		hasStaged, err = repo.HasStagedChanges()
		if err != nil {
			return err
		}
		if !hasStaged {
			return fmt.Errorf("staging files failed")
		}
	}

	diff, err := repo.StagedDiff()
	if err != nil {
		return err
	}

	agt := agent.Agent{
		Model:   cfg.Model,
		BaseURL: cfg.APIURL,
		Think:   cfg.Thinking,
		Client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}

	commitMsg, err := generateCommitMessage(agt, diff)
	if err != nil {
		return err
	}

	for {
		fmt.Println("\nSuggested commit message:")
		fmt.Printf("    %s\n", commitMsg)

		action, err := promptAction()
		if err != nil {
			return err
		}

		switch action {
		case "a", "accept":
			if err := repo.Commit(commitMsg); err != nil {
				return err
			}
			return nil
		case "r", "retry":
			commitMsg, err = generateCommitMessage(agt, diff)
			if err != nil {
				return err
			}
		case "c", "cancel":
			fmt.Println("Cancelled.")
			return nil
		default:
			fmt.Println("Please choose a, r, or c.")
		}
	}
}

func generateCommitMessage(agt agent.Agent, diff string) (string, error) {
	prompt := buildPrompt(diff)

	raw, err := agt.Generate(prompt)
	if err != nil {
		return "", err
	}

	msg, err := normalize(raw)
	if err != nil {
		return "", err
	}

	log.Println(msg)
	if err := validate(msg); err != nil {
		return "", fmt.Errorf("generated message is invalid: %w", err)
	}

	return msg, nil
}

func buildPrompt(diff string) string {
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

func normalize(msg string) (string, error) {
	msg = strings.TrimSpace(msg)

	for {
		prev := msg
		if len(msg) >= 2 && msg[0] == '"' && msg[len(msg)-1] == '"' {
			msg = msg[1 : len(msg)-1]
		}
		if len(msg) >= 2 && msg[0] == '\'' && msg[len(msg)-1] == '\'' {
			msg = msg[1 : len(msg)-1]
		}
		msg = strings.TrimPrefix(msg, "```")
		msg = strings.TrimSuffix(msg, "```")
		msg = strings.TrimPrefix(msg, "`")
		msg = strings.TrimSuffix(msg, "`")
		msg = strings.TrimSpace(msg)
		if msg == prev {
			break
		}
	}

	if idx := strings.Index(msg, "\n"); idx != -1 {
		msg = strings.TrimSpace(msg[:idx])
	}

	if msg == "" {
		return "", errEmptyResponse
	}

	return msg, nil
}

func validate(msg string) error {
	if msg == "" {
		return errors.New("commit message is empty")
	}
	if len(msg) > 150 {
		return fmt.Errorf("commit message exceeds 72 characters (%d)", len(msg))
	}

	col := strings.Index(msg, ":")
	if col < 1 {
		return errors.New("commit message does not follow Conventional Commits format")
	}

	prefix := strings.TrimSuffix(msg[:col], "!")
	if !isValidCCPrefix(prefix) {
		return errors.New("commit message does not follow Conventional Commits format")
	}
	return nil
}

func isValidCCPrefix(s string) bool {
	if !strings.Contains(s, "(") && !strings.Contains(s, ")") {
		return s != "" && !strings.ContainsAny(s, " \t")
	}
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 1 || close != len(s)-1 || close <= open+1 {
		return false
	}
	return !strings.ContainsAny(s[:open], " \t")
}

func handleUnstagedChanges(repo *git.Repo) error {
	files, err := repo.ChangedFiles()
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
		return repo.StageAll()
	case "select":
		selection, err := promptFileSelection(files)
		if err != nil {
			return err
		}
		if len(selection) == 0 {
			return ErrCancelled
		}
		return repo.StageFiles(selection)
	case "cancel":
		return ErrCancelled
	default:
		return fmt.Errorf("invalid staging action: %q", action)
	}
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

func promptFileSelection(files []git.File) ([]string, error) {
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

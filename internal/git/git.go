package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type File struct {
	Status string
	Path   string
}

type Repo struct {
	rootDir string
}

func NewRepo(rootDir string) *Repo {
	return &Repo{rootDir: rootDir}
}

func Available() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is not installed or not available in PATH")
	}
	return nil
}

func Root() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *Repo) HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = r.rootDir

	err := cmd.Run()
	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}

	return false, err
}

func (r *Repo) StagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--no-ext-diff")
	cmd.Dir = r.rootDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get staged diff error: %w", err)
	}

	return string(output), nil
}

func (r *Repo) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.rootDir
	return cmd.Run()
}

func (r *Repo) ChangedFiles() ([]File, error) {
	cmd := exec.Command("git", "-C", r.rootDir, "status", "--short")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get git status: %w", err)
	}

	var files []File
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		files = append(files, File{
			Status: strings.TrimSpace(line[:2]),
			Path:   line[3:],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read git status: %w", err)
	}

	return files, nil
}

func (r *Repo) StageAll() error {
	cmd := exec.Command("git", "-C", r.rootDir, "add", "-A")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stage all changes: %w", err)
	}
	return nil
}

func (r *Repo) StageFiles(filePaths []string) error {
	args := append([]string{"-C", r.rootDir, "add", "--"}, filePaths...)
	cmd := exec.Command("git", args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stage selected files: %w", err)
	}
	return nil
}
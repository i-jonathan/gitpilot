package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type gitFile struct {
	Status string
	Path   string
}

func gitAvailable() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is not installed or not available in PATH")
	}

	return nil
}

func gitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}

	return strings.TrimSpace(string(output)), nil
}

func hasStagedChanges(rootDir string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = rootDir

	err := cmd.Run()
	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 1 {
			return true, nil
		}
	}

	return false, err
}

func getStagedDiff(rootDir string) (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--no-ext-diff")
	cmd.Dir = rootDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get staged diff error: %w", err)
	}

	return string(output), nil
}

func commit(rootDir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = rootDir

	return cmd.Run()
}

func changedFiles(rootDir string) ([]gitFile, error) {
	cmd := exec.Command("git", "-C", rootDir, "status", "--short")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get git status: %w", err)
	}

	var files []gitFile
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) < 3 {
			continue
		}

		files = append(files, gitFile{
			Status: line[:2],
			Path:   line[3:],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read git status %w", err)
	}

	return files, nil
}

func stageAll(rootDir string) error {
	cmd := exec.Command("git", "-C", rootDir, "add", "-A")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stage all changes: %w", err)
	}

	return nil
}

func stageFiles(rootDir string, filePaths []string) error {
	args := []string{"-C", rootDir, "add", "--"}
	args = append(args, filePaths...)

	cmd := exec.Command("git", args...)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("stage selected files: %w", err)
	}

	return nil
}

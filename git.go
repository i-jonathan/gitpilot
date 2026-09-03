package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

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

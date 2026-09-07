package cli

import (
	"errors"
	"fmt"
	"os/exec"
)

func RequireCommand(command string) error {
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("required command %q not found in PATH", command)
	}
	return nil
}

func RequireGithubAuth() error {
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return errors.New("GitHub CLI is not authenticated; run `gh auth login`")
	}

	return nil
}

package cli

import (
	"fmt"
	"os/exec"
)

func RequireCommand(command string) error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("command: %s is not installed or not available in PATH", command)
	}
	return nil
}

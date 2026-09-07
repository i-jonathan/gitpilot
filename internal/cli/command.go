package cli

import (
	"fmt"
	"os/exec"
)

func RequireCommand(command string) error {
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("required command %q not found in PATH", command)
	}
	return nil
}

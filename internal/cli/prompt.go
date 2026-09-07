package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func PromptAction() (string, error) {
	fmt.Println("\n[a]ccept [r]etry [c]ancel")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.ToLower(strings.TrimSpace(input)), nil
}

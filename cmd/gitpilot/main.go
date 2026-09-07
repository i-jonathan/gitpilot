package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"gitpilot/internal/agent"
	"gitpilot/internal/cli"
	"gitpilot/internal/commit"
	"gitpilot/internal/config"
	"gitpilot/internal/git"
	"gitpilot/internal/pr"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}

	if err := cli.RequireCommand("git"); err != nil {
		fatal(err)
	}

	rootDir, err := git.Root()
	if err != nil {
		fatal(err)
	}

	repo := git.NewRepo(rootDir)

	agt := &agent.Agent{
		Model:   cfg.Model,
		BaseURL: cfg.APIURL,
		Think:   cfg.Thinking,
		Client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}

	switch os.Args[1] {
	case "commit":
		if err := commit.Run(repo, agt); err != nil {
			fatal(err)
		}
	case "pr":
		if err := pr.Run(repo, agt); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func usage() {
	fmt.Println(`Usage: gitpilot commit`)
}

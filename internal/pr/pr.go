package pr

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"gitpilot/internal/agent"
	"gitpilot/internal/cli"
	"gitpilot/internal/git"
	"os"
	"os/exec"
	"strings"
)

type PullRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func Run(repo *git.Repo, agent *agent.Agent) error {
	err := cli.RequireCommand("gh")
	if err != nil {
		return err
	}

	current, err := repo.CurrentBranch()
	if err != nil {
		return err
	}

	base, err := repo.DefaultBranch()
	if err != nil {
		return err
	}

	if base == current {
		return errors.New("cannot create a pull request from the default branch")
	}

	pushed, err := repo.HasRemoteBranch(current)
	if err != nil {
		return err
	}

	if !pushed {
		return fmt.Errorf("branch %q has not been pushed to origin", current)
	}

	commits, err := repo.CommitsSince(base)
	if err != nil {
		return err
	}

	diff, err := repo.DiffSince(base)
	if err != nil {
		return err
	}

	if strings.TrimSpace(commits) == "" {
		return errors.New("no commits found since the default branch")
	}

	prompt := buildPrompt(commits, diff)

	pullRequest, err := generate(agent, prompt)
	if err != nil {
		return err
	}

	err = pullRequest.normalize()
	if err != nil {
		return err
	}

	pullRequest.print()

	for {
		action, err := promptAction()
		if err != nil {
			return err
		}

		switch action {
		case "a", "accept":
			url, err := createPR(repo, pullRequest)
			if err != nil {
				return err
			}

			fmt.Println("✓ Pull Request Created")
			fmt.Println(url)
			return nil
		case "r", "retry":
			pullRequest, err := generate(agent, prompt)
			if err != nil {
				return err
			}

			err = pullRequest.normalize()
			if err != nil {
				return err
			}

			pullRequest.print()
			continue
		case "c", "cancel":
			fmt.Println("Cancelled")
			return nil
		default:
			fmt.Println("Please enter a, r, or c.")
		}
	}
}

func buildPrompt(commits, diff string) string {
	return fmt.Sprintf(`You are an expert software engineer generating a GitHub pull request.
		Analyze the commits and code changes below and generate a concise pull request title and description.

		Rules:
		- Return valid JSON only.
		- Format:
		  {
		    "title": "...",
		    "description": "..."
		  }
		- The title should be concise and describe the overall purpose of the changes.
		- Use Conventional Commits format for the title when appropriate.
		- Keep the title concise and under 100 characters.
		- The description should explain what changed and why.
		- Use Markdown in the description where helpful.
		- Do not invent information.
		- Do not include reasoning outside the JSON.

		Commits:
		%s

		Diff:
		%s
		`,
		commits, diff,
	)
}

func generate(agent *agent.Agent, prompt string) (PullRequest, error) {
	response, err := agent.Generate(prompt)
	if err != nil {
		return PullRequest{}, fmt.Errorf("generate pull request error: %w", err)
	}

	var pull PullRequest
	if err := json.Unmarshal([]byte(response), &pull); err != nil {
		return PullRequest{}, fmt.Errorf("parse pull request response: %w", err)
	}

	return pull, nil
}

func (p *PullRequest) normalize() error {
	p.Title = strings.TrimSpace(p.Title)
	p.Description = strings.TrimSpace(p.Description)

	if p.Title == "" {
		return errors.New("generated PR title is empty")
	}

	if p.Description == "" {
		return errors.New("generated PR description is empty")
	}

	if len([]rune(p.Title)) > 100 {
		return errors.New("generated PR title exceeds 100 characters")
	}

	return nil
}

func (p *PullRequest) print() {
	fmt.Println("Pull Request:")
	fmt.Printf("Title: %s\n\n", p.Title)
	fmt.Println("Description:")
	fmt.Println(p.Description)
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

func createPR(repo *git.Repo, pull PullRequest) (string, error) {
	cmd := exec.Command(
		"gh", "pr", "create",
		"--title", pull.Title,
		"--body", pull.Description,
	)
	cmd.Dir = repo.RootDir()

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("create pull request: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

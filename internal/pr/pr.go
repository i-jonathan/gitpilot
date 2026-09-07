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

type ParseError struct {
	Response string
	Err      error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse pull request response: %v", e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

var pullRequestSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title": map[string]any{
			"type": "string",
		},
		"description": map[string]any{
			"type": "string",
		},
	},
	"required": []string{"title", "description"},
}

func Run(repo *git.Repo, agent *agent.Agent) error {
	err := cli.RequireCommand("gh")
	if err != nil {
		return err
	}

	err = cli.RequireGithubAuth()
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

	pullRequest, err := generateWithRetry(agent, prompt)
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
	return fmt.Sprintf(`Generate a GitHub pull request from the commits and code changes below.

	OUTPUT FORMAT:
	Return exactly one JSON object and nothing else.

	{
  "title": "pull request title",
  "description": "pull request description"
	}

	STRICT RULES:
	- Your response MUST start with {
	- Your response MUST end with }
	- Do NOT use Markdown code fences.
	- Do NOT include explanations, analysis, reasoning, or commentary.
	- Do NOT include any text before or after the JSON.
	- The title must be concise and under 100 characters.
	- Use Conventional Commits format for the title when appropriate.
	- The description should explain what changed and why.
	- Use Markdown in the description where helpful.
	- Do not invent information.

	COMMITS:
	%s

	CODE CHANGES:
	%s
	`,
		commits, diff,
	)
}

func generate(a *agent.Agent, prompt string) (PullRequest, error) {
	response, err := a.GenerateWithOptions(prompt, agent.GenerateOptions{
		Format: pullRequestSchema,
	})

	if err != nil {
		return PullRequest{}, fmt.Errorf("generate pull request error: %w", err)
	}

	var pull PullRequest
	if err := json.Unmarshal([]byte(response), &pull); err != nil {
		return PullRequest{}, &ParseError{
			Response: response,
			Err:      err,
		}
	}

	return pull, nil
}

func generateWithRetry(agent *agent.Agent, prompt string) (PullRequest, error) {
	pullRequest, err := generate(agent, prompt)
	if err == nil {
		return pullRequest, nil
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		return PullRequest{}, err
	}

	fmt.Println("⚠ Structured response failed. Retrying...")

	pullRequest, retryErr := generate(agent, prompt)
	if retryErr == nil {
		return pullRequest, nil
	}

	var retryParseErr *ParseError
	if errors.As(retryErr, &retryParseErr) {
		printParseFailure(retryParseErr)
	}

	return PullRequest{}, retryErr
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

func printParseFailure(err *ParseError) {
	fmt.Println()
	fmt.Println("⚠ The model failed to return a structured pull request response.")
	fmt.Println("The selected model may not reliably support structured output.")
	fmt.Println()
	fmt.Println("Raw model response:")
	fmt.Println("────────────────────────────────────────")
	fmt.Println(err.Response)
	fmt.Println("────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Try another model or use the response above manually.")
}

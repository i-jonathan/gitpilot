package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoFromTestRepo(t *testing.T) *Repo {
	t.Helper()
	return NewRepo(createTestRepo(t))
}

func TestAvailable(t *testing.T) {
	if err := Available(); err != nil {
		t.Fatal(err)
	}
}

func TestHasStagedChanges_NoChangesInitially(t *testing.T) {
	repo := repoFromTestRepo(t)
	ok, err := repo.HasStagedChanges()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected no staged changes in fresh repo")
	}
}

func TestStagedDiff_Smoke(t *testing.T) {
	repo := repoFromTestRepo(t)
	diff, err := repo.StagedDiff()
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Errorf("expected empty diff, got %q", diff[:min(len(diff), 40)])
	}
}

func TestStagedChangesAfterStaging(t *testing.T) {
	repo := repoFromTestRepo(t)

	if err := os.WriteFile(filepath.Join(repo.rootDir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := repo.StageAll(); err != nil {
		t.Fatal(err)
	}

	ok, err := repo.HasStagedChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected staged changes after staging")
	}

	diff, err := repo.StagedDiff()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "test.txt") {
		t.Errorf("diff should mention test.txt, got: %s", diff[:min(len(diff), 100)])
	}
}

func TestCommit(t *testing.T) {
	repo := repoFromTestRepo(t)

	if err := os.WriteFile(filepath.Join(repo.rootDir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := repo.StageAll(); err != nil {
		t.Fatal(err)
	}

	if err := repo.Commit("feat: add test.txt"); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "-C", repo.rootDir, "log", "--oneline", "-1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "feat: add test.txt") {
		t.Errorf("commit message not found in log: %s", out)
	}
}

func TestChangedFiles_EmptyRepo(t *testing.T) {
	repo := repoFromTestRepo(t)
	files, err := repo.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected no changed files, got %d", len(files))
	}
}

func TestChangedFiles_AfterModification(t *testing.T) {
	repo := repoFromTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo.rootDir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := repo.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 changed file, got %d", len(files))
	}
	if files[0].Status != "??" {
		t.Errorf("expected status '??', got %q", files[0].Status)
	}
	if files[0].Path != "test.txt" {
		t.Errorf("expected path 'test.txt', got %q", files[0].Path)
	}
}

func TestStageFiles_Selected(t *testing.T) {
	repo := repoFromTestRepo(t)

	if err := os.WriteFile(filepath.Join(repo.rootDir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.rootDir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := repo.StageFiles([]string{"a.txt"}); err != nil {
		t.Fatal(err)
	}

	ok, err := repo.HasStagedChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected staged changes after staging one file")
	}

	diff, err := repo.StagedDiff()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "a.txt") {
		t.Errorf("diff should mention a.txt, got: %s", diff[:min(len(diff), 100)])
	}
	if strings.Contains(diff, "b.txt") {
		t.Errorf("diff should NOT mention b.txt, got: %s", diff[:min(len(diff), 100)])
	}
}

func TestStageAll_Error(t *testing.T) {
	repo := NewRepo(t.TempDir())
	err := repo.StageAll()
	if err == nil {
		t.Fatal("expected error staging in non-git directory")
	}
	if !strings.Contains(err.Error(), "stage all changes") {
		t.Errorf("got %v, want 'stage all changes' error", err)
	}
}

func TestStageFiles_Error(t *testing.T) {
	repo := NewRepo(t.TempDir())
	err := repo.StageFiles([]string{"foo.txt"})
	if err == nil {
		t.Fatal("expected error staging files in non-git directory")
	}
	if !strings.Contains(err.Error(), "stage selected files") {
		t.Errorf("got %v, want 'stage selected files' error", err)
	}
}

func TestHasStagedChanges_Error(t *testing.T) {
	repo := NewRepo(t.TempDir())
	_, err := repo.HasStagedChanges()
	if err == nil {
		t.Fatal("expected error in non-git directory")
	}
}

func TestStagedDiff_Error(t *testing.T) {
	repo := NewRepo(t.TempDir())
	_, err := repo.StagedDiff()
	if err == nil {
		t.Fatal("expected error in non-git directory")
	}
	if !strings.Contains(err.Error(), "get staged diff error") {
		t.Errorf("got %v, want 'get staged diff error'", err)
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := repoFromTestRepo(t)
	branch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "main")
	}
}

func TestCommitsSince(t *testing.T) {
	repo := repoFromTestRepo(t)

	// Create and switch to a feature branch to have a real commit range
	cmd := exec.Command("git", "-C", repo.rootDir, "checkout", "-b", "feature")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repo.rootDir, "feature.txt"), []byte("feature"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", repo.rootDir, "add", "feature.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", repo.rootDir, "commit", "-m", "feat: add feature")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	commits, err := repo.CommitsSince("main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(commits, "feat: add feature") {
		t.Errorf("CommitsSince should include 'feat: add feature', got: %s", commits)
	}
	if strings.Contains(commits, "initial") {
		t.Errorf("CommitsSince from main should not include the initial commit, got: %s", commits)
	}
}

func TestDiffSince(t *testing.T) {
	repo := repoFromTestRepo(t)

	// Create and switch to a feature branch to have a real diff range
	cmd := exec.Command("git", "-C", repo.rootDir, "checkout", "-b", "feature")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repo.rootDir, "feature.txt"), []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", repo.rootDir, "add", "feature.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", repo.rootDir, "commit", "-m", "feat: add feature")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	diff, err := repo.DiffSince("main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "feature.txt") {
		t.Errorf("DiffSince should mention feature.txt, got: %s", diff[:min(len(diff), 200)])
	}
	if !strings.Contains(diff, "new content") {
		t.Errorf("DiffSince should contain the new content, got: %s", diff[:min(len(diff), 200)])
	}
}

func TestRootDir(t *testing.T) {
	repo := repoFromTestRepo(t)
	dir := repo.RootDir()
	if dir == "" {
		t.Error("RootDir() returned empty string")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("RootDir() = %q, but directory does not exist: %v", dir, err)
	}
}

func TestRoot(t *testing.T) {
	repo := repoFromTestRepo(t)

	savedDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(savedDir)

	if err := os.Chdir(repo.rootDir); err != nil {
		t.Fatal(err)
	}

	got, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	// git rev-parse --show-toplevel resolves symlinks, returning the real path.
	// On macOS, /tmp is a symlink to /private/tmp and /var to /private/var.
	// Compare by evaluating the trailing portion of the path.
	if !strings.HasSuffix(got, "TestRoot") && !strings.HasSuffix(repo.rootDir, filepath.Base(string(got))) {
		t.Errorf("Root() = %q, want suffix matching %q", got, repo.rootDir)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("Root() = %q, but directory does not exist: %v", got, err)
	}
}

func TestRoot_NotInGitRepo(t *testing.T) {
	savedDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(savedDir)

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	_, err = Root()
	if err == nil {
		t.Fatal("expected error when not in a git repository")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("got %q, want 'not inside a git repository'", err)
	}
}

func TestHasRemoteBranch_Error(t *testing.T) {
	repo := NewRepo(t.TempDir())
	_, err := repo.HasRemoteBranch("main")
	if err == nil {
		t.Fatal("expected error in non-git directory")
	}
	if !strings.Contains(err.Error(), "check remote branch") {
		t.Errorf("got %v, want 'check remote branch'", err)
	}
}

func TestChangedFiles_Error(t *testing.T) {
	repo := NewRepo(t.TempDir())
	_, err := repo.ChangedFiles()
	if err == nil {
		t.Fatal("expected error in non-git directory")
	}
	if !strings.Contains(err.Error(), "get git status") {
		t.Errorf("got %v, want 'get git status' error", err)
	}
}

func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmd := exec.Command("git", "-C", dir, "init", "--initial-branch", "main")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@test")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@test")

	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", dir, "add", "-A")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "initial")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}
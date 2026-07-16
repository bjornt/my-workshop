// Package gitenv provides test helpers that drive real git repositories.
// It replaces the pytest fixtures git_repo/in_tmp and the helpers from
// tests/git.py. It intentionally does not import packages under test.
package gitenv

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// RunGit runs a git command in cwd and returns its stdout. The test fails if
// the command exits non-zero.
func RunGit(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("git %v in %s failed (exit %d): %s%s", args, cwd, exitErr.ExitCode(), string(out), string(exitErr.Stderr))
		}
		t.Fatalf("git %v in %s failed: %v", args, cwd, err)
	}
	return string(out)
}

// CommitFile writes name with content, adds it, commits it, and returns the
// absolute path to the file.
func CommitFile(t *testing.T, repo string, name string, content string) string {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	RunGit(t, repo, "add", "--", name)
	RunGit(t, repo, "commit", "-m", fmt.Sprintf("add %s", name))
	return path
}

// SkipWorktreeBit returns the first character of `git ls-files -v -- name`,
// e.g. "S" when the skip-worktree bit is set. It returns "" if there is no
// output.
func SkipWorktreeBit(t *testing.T, repo string, name string) string {
	t.Helper()
	out := RunGit(t, repo, "ls-files", "-v", "--", name)
	if out == "" {
		return ""
	}
	return out[:1]
}

// ExcludeLines returns the lines currently in .git/info/exclude.
func ExcludeLines(repo string) []string {
	path := filepath.Join(repo, ".git", "info", "exclude")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

// NewRepo creates a real git repository, changes into it, and arranges for the
// working directory to be restored after the test. It is the Go equivalent of
// the git_repo pytest fixture.
func NewRepo(t *testing.T) string {
	t.Helper()

	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, []byte{}, 0o644); err != nil {
		t.Fatalf("create empty gitconfig: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	repo := t.TempDir()
	RunGit(t, repo, "init", "-b", "main")
	RunGit(t, repo, "config", "user.email", "dev@example.com")
	RunGit(t, repo, "config", "user.name", "Dev")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	return repo
}

// NewTmp changes into an empty temporary directory and arranges for the
// working directory to be restored after the test. It is the Go equivalent of
// the in_tmp pytest fixture.
func NewTmp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	return dir
}

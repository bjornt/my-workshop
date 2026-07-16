package worktree_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bjornt/my-workshop/internal/testsupport/gitenv"
	"github.com/bjornt/my-workshop/internal/worktree"
)

const prog = "my-workshop"

type logCapture struct {
	logs []string
}

func (l *logCapture) append(s string) {
	l.logs = append(l.logs, s)
}

func (l *logCapture) contains(sub string) bool {
	for _, m := range l.logs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

func porcelain(t *testing.T, repo string) string {
	t.Helper()
	return gitenv.RunGit(t, repo, "status", "--porcelain")
}

func TestWorktree(t *testing.T) {
	t.Run("hide tracked sets skip-worktree and hides local edits", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		path := gitenv.CommitFile(t, repo, "workshop.yaml", "name: dev\n")
		var cap logCapture

		worktree.HideInWorktree("workshop.yaml", prog, cap.append)

		if got := gitenv.SkipWorktreeBit(t, repo, "workshop.yaml"); got != "S" {
			t.Fatalf("skip-worktree bit = %q, want S", got)
		}

		if err := os.WriteFile(path, []byte("name: LOCALLY EDITED\n"), 0o644); err != nil {
			t.Fatalf("edit file: %v", err)
		}
		if strings.Contains(porcelain(t, repo), "workshop.yaml") {
			t.Fatalf("workshop.yaml should not appear in git status")
		}
	})

	t.Run("hide tracked is idempotent", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		gitenv.CommitFile(t, repo, "workshop.yaml", "name: dev\n")
		var cap logCapture

		worktree.HideInWorktree("workshop.yaml", prog, cap.append)
		worktree.HideInWorktree("workshop.yaml", prog, cap.append)

		if got := gitenv.SkipWorktreeBit(t, repo, "workshop.yaml"); got != "S" {
			t.Fatalf("skip-worktree bit = %q, want S", got)
		}
	})

	t.Run("revert tracked clears bit and restores committed content", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		path := gitenv.CommitFile(t, repo, "workshop.yaml", "name: dev\n")
		var cap logCapture
		worktree.HideInWorktree("workshop.yaml", prog, cap.append)
		if got := gitenv.SkipWorktreeBit(t, repo, "workshop.yaml"); got != "S" {
			t.Fatalf("skip-worktree bit = %q, want S", got)
		}

		if err := os.WriteFile(path, []byte("name: LOCALLY EDITED\n"), 0o644); err != nil {
			t.Fatalf("edit file: %v", err)
		}

		worktree.Revert("workshop.yaml", prog, cap.append)

		if got := gitenv.SkipWorktreeBit(t, repo, "workshop.yaml"); got == "S" {
			t.Fatalf("skip-worktree bit still set")
		}
		if worktree.SkipWorktreeSet("workshop.yaml") {
			t.Fatalf("SkipWorktreeSet still true")
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if string(content) != "name: dev\n" {
			t.Fatalf("content = %q, want committed content", string(content))
		}
	})

	t.Run("hide untracked excludes anchored pattern and drops from status", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		path := filepath.Join(repo, "workshop.yaml")
		if err := os.WriteFile(path, []byte("name: dev\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if !strings.Contains(porcelain(t, repo), "workshop.yaml") {
			t.Fatalf("workshop.yaml should appear in git status before hiding")
		}
		var cap logCapture

		worktree.HideInWorktree("workshop.yaml", prog, cap.append)

		lines := gitenv.ExcludeLines(repo)
		found := false
		for _, line := range lines {
			if line == "/workshop.yaml" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("/workshop.yaml not in exclude lines: %v", lines)
		}
		if strings.Contains(porcelain(t, repo), "workshop.yaml") {
			t.Fatalf("workshop.yaml should not appear in git status after hiding")
		}
	})

	t.Run("hide untracked does not duplicate exclude line", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		path := filepath.Join(repo, "workshop.yaml")
		if err := os.WriteFile(path, []byte("name: dev\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		var cap logCapture

		worktree.HideInWorktree("workshop.yaml", prog, cap.append)
		worktree.HideInWorktree("workshop.yaml", prog, cap.append)

		count := 0
		for _, line := range gitenv.ExcludeLines(repo) {
			if line == "/workshop.yaml" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("/workshop.yaml appears %d times, want 1", count)
		}
	})

	t.Run("revert untracked removes exclude line but keeps file", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		path := filepath.Join(repo, "workshop.yaml")
		if err := os.WriteFile(path, []byte("name: dev\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		var cap logCapture
		worktree.HideInWorktree("workshop.yaml", prog, cap.append)
		found := false
		for _, line := range gitenv.ExcludeLines(repo) {
			if line == "/workshop.yaml" {
				found = true
			}
		}
		if !found {
			t.Fatalf("/workshop.yaml should be in exclude lines before revert")
		}

		worktree.Revert("workshop.yaml", prog, cap.append)

		for _, line := range gitenv.ExcludeLines(repo) {
			if line == "/workshop.yaml" {
				t.Fatalf("/workshop.yaml should be removed from exclude lines")
			}
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file should still exist: %v", err)
		}
	})

	t.Run("predicates report correct state inside repo", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		if !worktree.InGitRepo() {
			t.Fatalf("InGitRepo = false, want true")
		}

		gitenv.CommitFile(t, repo, "tracked.txt", "hi\n")
		if !worktree.GitTracked("tracked.txt") {
			t.Fatalf("GitTracked = false, want true")
		}
		if worktree.SkipWorktreeSet("tracked.txt") {
			t.Fatalf("SkipWorktreeSet = true, want false")
		}

		if err := os.WriteFile(filepath.Join(repo, "loose.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write loose file: %v", err)
		}
		if worktree.GitTracked("loose.txt") {
			t.Fatalf("GitTracked = true, want false")
		}

		var cap logCapture
		worktree.HideInWorktree("tracked.txt", prog, cap.append)
		if !worktree.SkipWorktreeSet("tracked.txt") {
			t.Fatalf("SkipWorktreeSet = false, want true")
		}
	})

	t.Run("outside repo hide is silent noop and revert reports", func(t *testing.T) {
		_ = gitenv.NewTmp(t)
		if worktree.InGitRepo() {
			t.Fatalf("InGitRepo = true outside repo")
		}

		var hideCap logCapture
		worktree.HideInWorktree("workshop.yaml", prog, hideCap.append)
		if len(hideCap.logs) != 0 {
			t.Fatalf("hide should log nothing outside repo, got %v", hideCap.logs)
		}
		if _, err := os.Stat("workshop.yaml"); !os.IsNotExist(err) {
			t.Fatalf("hide should not create workshop.yaml outside repo")
		}

		var revertCap logCapture
		worktree.Revert("workshop.yaml", prog, revertCap.append)
		if !revertCap.contains("not in a git repository") {
			t.Fatalf("revert should report not in a git repository, got %v", revertCap.logs)
		}
	})

	t.Run("revert plain untracked file reports nothing to revert", func(t *testing.T) {
		repo := gitenv.NewRepo(t)
		path := filepath.Join(repo, "plain.txt")
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		var cap logCapture
		worktree.Revert("plain.txt", prog, cap.append)

		if !cap.contains("not ignored by " + prog) {
			t.Fatalf("revert should report nothing to revert, got %v", cap.logs)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file should still exist: %v", err)
		}
	})
}

// Package worktree hides a workshop YAML file from git using either the
// skip-worktree bit (tracked files) or .git/info/exclude (untracked files).
// It shells out to the real git binary and degrades to a no-op when git is
// absent or the current directory is outside a repository.
package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Logger is the seam used for side-effect logging.
type Logger func(string)

// git runs a git command and returns its stdout, exit code, and whether the
// git binary was found. If git is missing, ok is false.
func git(args ...string) (stdout string, code int, ok bool) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", 0, false
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode(), true
		}
		return "", 0, false
	}
	return string(out), 0, true
}

// gitOK runs a git command and reports whether it succeeded.
func gitOK(args ...string) bool {
	_, code, ok := git(args...)
	return ok && code == 0
}

// GitTracked reports whether path is tracked by git in the current repo.
func GitTracked(path string) bool {
	_, code, ok := git("ls-files", "--error-unmatch", "--", path)
	return ok && code == 0
}

// SkipWorktreeSet reports whether path currently has the skip-worktree bit.
func SkipWorktreeSet(path string) bool {
	out, code, ok := git("ls-files", "-v", "--", path)
	if !ok || code != 0 {
		return false
	}
	if out == "" {
		return false
	}
	return out[:1] == "S"
}

// InGitRepo reports whether the current directory is inside a git work tree.
func InGitRepo() bool {
	out, code, ok := git("rev-parse", "--is-inside-work-tree")
	return ok && code == 0 && strings.TrimSpace(out) == "true"
}

// gitPath returns the absolute path to a file inside the git directory,
// or ("", false) when git is unavailable or the path cannot be resolved.
func gitPath(name string) (string, bool) {
	out, code, ok := git("rev-parse", "--git-path", name)
	if !ok || code != 0 {
		return "", false
	}
	return strings.TrimSpace(out), true
}

// excludePattern returns the anchored .gitignore pattern for path relative to
// the repo top-level, or ("", false) if path lies outside the repository.
func excludePattern(path string) (string, bool) {
	top, code, ok := git("rev-parse", "--show-toplevel")
	if !ok || code != 0 {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(strings.TrimSpace(top), abs)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return "/" + strings.ReplaceAll(rel, string(filepath.Separator), "/"), true
}

// excludeLines splits exclude file content into lines like Python's
// splitlines() (no trailing empty line after a final newline).
func excludeLinesFromContent(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

// ExcludeAdd adds path's anchored ignore pattern to .git/info/exclude.
// It returns true if the pattern is present afterwards.
func ExcludeAdd(path string) bool {
	pattern, ok := excludePattern(path)
	if !ok {
		return false
	}
	excl, ok := gitPath("info/exclude")
	if !ok {
		return false
	}

	var content string
	if data, err := os.ReadFile(excl); err == nil {
		content = string(data)
	}

	for _, line := range excludeLinesFromContent(content) {
		if line == pattern {
			return true
		}
	}

	if err := os.MkdirAll(filepath.Dir(excl), 0o755); err != nil {
		return false
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(excl, []byte(content+pattern+"\n"), 0o644); err != nil {
		return false
	}
	return true
}

// ExcludeRemove removes path's anchored ignore pattern from .git/info/exclude.
// It returns true if a matching line was removed.
func ExcludeRemove(path string) bool {
	pattern, ok := excludePattern(path)
	if !ok {
		return false
	}
	excl, ok := gitPath("info/exclude")
	if !ok {
		return false
	}
	data, err := os.ReadFile(excl)
	if err != nil {
		return false
	}

	lines := excludeLinesFromContent(string(data))
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != pattern {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return false
	}

	out := strings.Join(kept, "\n")
	if len(kept) > 0 {
		out += "\n"
	}
	if err := os.WriteFile(excl, []byte(out), 0o644); err != nil {
		return false
	}
	return true
}

// HideInWorktree hides path from git: tracked files get the skip-worktree bit,
// untracked files are added to .git/info/exclude. It is a silent no-op outside
// a git repository.
func HideInWorktree(path string, prog string, log Logger) {
	if !InGitRepo() {
		return
	}
	if GitTracked(path) {
		if !SkipWorktreeSet(path) {
			gitOK("update-index", "--skip-worktree", "--", path)
		}
		log(fmt.Sprintf(
			"Ignoring local changes to %s in the work tree (git skip-worktree).\n"+
				"  'git status' won't show it as modified and 'git commit -a' skips it.\n"+
				"  To restore the tracked file or make a committable change: %s --revert",
			path, prog,
		))
	} else if ExcludeAdd(path) {
		log(fmt.Sprintf(
			"Ignoring %s in the work tree (.git/info/exclude).\n"+
				"  'git status' won't list it under untracked files. Local to this\n"+
				"  work tree only; never committed or pushed.\n"+
				"  To stop ignoring it: %s --revert",
			path, prog,
		))
	}
}

// Revert undoes HideInWorktree for path. Outside a git repository it logs that
// there is nothing to revert.
func Revert(path string, prog string, log Logger) {
	if !InGitRepo() {
		log(fmt.Sprintf("%s is not in a git repository; nothing to revert.", path))
		return
	}
	if GitTracked(path) {
		if SkipWorktreeSet(path) {
			gitOK("update-index", "--no-skip-worktree", "--", path)
			log(fmt.Sprintf("Stopped ignoring %s in the work tree (skip-worktree cleared).", path))
		}
		gitOK("checkout", "--", path)
		log(fmt.Sprintf("Restored %s to the version tracked in git.", path))
	} else if ExcludeRemove(path) {
		log(fmt.Sprintf("Stopped ignoring %s in the work tree (.git/info/exclude cleared).", path))
	} else {
		log(fmt.Sprintf("%s is not ignored by %s; nothing to revert.", path, prog))
	}
}

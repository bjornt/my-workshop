"""Helpers for driving the real `git` binary in tests.

Nothing here is mocked: every function shells out to the actual `git` command so
tests observe real repository state.
"""

import os
import subprocess


def run_git(*args, cwd):
    """Run a git command in `cwd`; return the CompletedProcess."""
    return subprocess.run(
        ["git", *args], cwd=cwd, check=True, capture_output=True, text=True
    )


def git_commit_file(repo, name, content):
    """Write, add, and commit a file; return its path."""
    path = os.path.join(repo, name)
    with open(path, "w") as f:
        f.write(content)
    run_git("add", "--", name, cwd=repo)
    run_git("commit", "-m", f"add {name}", cwd=repo)
    return path


def skip_worktree_bit(repo, name):
    """Return the ls-files -v status char for name (e.g. 'S' when skipped)."""
    out = run_git("ls-files", "-v", "--", name, cwd=repo).stdout
    return out[:1] if out else ""


def exclude_lines(repo):
    """Return the lines currently in .git/info/exclude."""
    path = os.path.join(repo, ".git", "info", "exclude")
    if not os.path.exists(path):
        return []
    with open(path) as f:
        return f.read().splitlines()

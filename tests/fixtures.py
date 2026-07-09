"""Reusable pytest fixtures.

`git_repo` builds a real temporary git repository and chdirs into it so the
worktree helpers exercise the actual `git` binary rather than a mock. `in_tmp`
just chdirs into an empty temp dir for the pure YAML-editing tests.
"""

import pytest

from tests.git import run_git


@pytest.fixture
def in_tmp(tmp_path, monkeypatch):
    """Chdir into an empty temp directory (no git)."""
    monkeypatch.chdir(tmp_path)
    return tmp_path


@pytest.fixture
def git_repo(tmp_path, monkeypatch):
    """A real, initialized git repo with committer identity, chdir'd into."""
    run_git("init", "-b", "main", cwd=tmp_path)
    run_git("config", "user.email", "dev@example.com", cwd=tmp_path)
    run_git("config", "user.name", "Dev", cwd=tmp_path)
    monkeypatch.chdir(tmp_path)
    return tmp_path

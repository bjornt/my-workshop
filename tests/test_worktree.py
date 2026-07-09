"""Tests for my_workshop.worktree against a REAL git repository.

Every git interaction here goes through the actual `git` binary via the
`git_repo`/`in_tmp` fixtures -- nothing is mocked or stubbed. The observable
contracts under test are what a user sees: whether an edited file shows up in
`git status --porcelain`, what lands in `.git/info/exclude`, whether committed
content is restored on revert, and the messages logged on the no-op paths.
`git status --porcelain` is run through subprocess in-test to prove the
visibility effect independently of the module under test.
"""

import os
import subprocess

from my_workshop.worktree import (
    git_tracked,
    hide_in_worktree,
    in_git_repo,
    revert,
    skip_worktree_set,
)
from tests.git import exclude_lines, git_commit_file, skip_worktree_bit

PROG = "my-workshop"


def _porcelain(repo):
    """Real `git status --porcelain` output, run in-test (not via the module)."""
    return subprocess.run(
        ["git", "status", "--porcelain"],
        cwd=repo,
        capture_output=True,
        text=True,
        check=True,
    ).stdout


# --- tracked files: skip-worktree bit ---------------------------------------


def test_hide_tracked_sets_skip_worktree_and_hides_local_edits(git_repo):
    repo = git_repo
    path = git_commit_file(repo, "workshop.yaml", "name: dev\n")

    hide_in_worktree("workshop.yaml", PROG, log=[].append)

    # The skip-worktree bit is set...
    assert skip_worktree_bit(repo, "workshop.yaml") == "S"

    # ...so a local edit is invisible to `git status`.
    with open(path, "w") as f:
        f.write("name: LOCALLY EDITED\n")
    assert "workshop.yaml" not in _porcelain(repo)


def test_hide_tracked_is_idempotent(git_repo):
    repo = git_repo
    git_commit_file(repo, "workshop.yaml", "name: dev\n")

    hide_in_worktree("workshop.yaml", PROG, log=[].append)
    # Hiding a second time must not raise and must leave the bit set.
    hide_in_worktree("workshop.yaml", PROG, log=[].append)

    assert skip_worktree_bit(repo, "workshop.yaml") == "S"


def test_revert_tracked_clears_bit_and_restores_committed_content(git_repo):
    repo = git_repo
    path = git_commit_file(repo, "workshop.yaml", "name: dev\n")
    hide_in_worktree("workshop.yaml", PROG, log=[].append)
    assert skip_worktree_bit(repo, "workshop.yaml") == "S"

    # Make a local edit that revert must discard.
    with open(path, "w") as f:
        f.write("name: LOCALLY EDITED\n")

    revert("workshop.yaml", PROG, log=[].append)

    # The 'S' bit is cleared...
    assert skip_worktree_bit(repo, "workshop.yaml") != "S"
    assert skip_worktree_set("workshop.yaml") is False
    # ...and the committed content is restored, discarding the local edit.
    with open(path) as f:
        assert f.read() == "name: dev\n"


# --- untracked files: .git/info/exclude -------------------------------------


def test_hide_untracked_excludes_anchored_pattern_and_drops_from_status(git_repo):
    repo = git_repo
    path = os.path.join(repo, "workshop.yaml")
    with open(path, "w") as f:
        f.write("name: dev\n")

    # Baseline: an untracked file shows up in the porcelain listing.
    assert "workshop.yaml" in _porcelain(repo)

    hide_in_worktree("workshop.yaml", PROG, log=[].append)

    # An anchored '/name' pattern is added and the file drops off the list.
    assert "/workshop.yaml" in exclude_lines(repo)
    assert "workshop.yaml" not in _porcelain(repo)


def test_hide_untracked_does_not_duplicate_exclude_line(git_repo):
    repo = git_repo
    with open(os.path.join(repo, "workshop.yaml"), "w") as f:
        f.write("name: dev\n")

    hide_in_worktree("workshop.yaml", PROG, log=[].append)
    hide_in_worktree("workshop.yaml", PROG, log=[].append)

    assert exclude_lines(repo).count("/workshop.yaml") == 1


def test_revert_untracked_removes_exclude_line_but_keeps_file(git_repo):
    repo = git_repo
    path = os.path.join(repo, "workshop.yaml")
    with open(path, "w") as f:
        f.write("name: dev\n")
    hide_in_worktree("workshop.yaml", PROG, log=[].append)
    assert "/workshop.yaml" in exclude_lines(repo)

    revert("workshop.yaml", PROG, log=[].append)

    assert "/workshop.yaml" not in exclude_lines(repo)
    # git never had a version to restore, so the file stays on disk.
    assert os.path.exists(path)


# --- repo-state predicates --------------------------------------------------


def test_predicates_report_correct_state_inside_repo(git_repo):
    repo = git_repo
    assert in_git_repo() is True

    git_commit_file(repo, "tracked.txt", "hi\n")
    assert git_tracked("tracked.txt") is True
    assert skip_worktree_set("tracked.txt") is False

    with open(os.path.join(repo, "loose.txt"), "w") as f:
        f.write("x\n")
    assert git_tracked("loose.txt") is False

    hide_in_worktree("tracked.txt", PROG, log=[].append)
    assert skip_worktree_set("tracked.txt") is True


# --- outside any git repository ---------------------------------------------


def test_outside_repo_hide_is_silent_noop_and_revert_reports(in_tmp):
    assert in_git_repo() is False

    hide_logs = []
    hide_in_worktree("workshop.yaml", PROG, log=hide_logs.append)
    # Silent no-op: nothing logged, no file conjured into existence.
    assert hide_logs == []
    assert not os.path.exists("workshop.yaml")

    revert_logs = []
    revert("workshop.yaml", PROG, log=revert_logs.append)
    assert any("not in a git repository" in m for m in revert_logs)


# --- nothing-to-revert path -------------------------------------------------


def test_revert_plain_untracked_file_reports_nothing_to_revert(git_repo):
    repo = git_repo
    path = os.path.join(repo, "plain.txt")
    with open(path, "w") as f:
        f.write("x\n")

    logs = []
    revert("plain.txt", PROG, log=logs.append)

    assert any(f"not ignored by {PROG}" in m for m in logs)
    assert os.path.exists(path)

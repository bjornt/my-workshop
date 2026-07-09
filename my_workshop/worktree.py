"""Hide the workshop YAML from git so local edits never show in 'git status'.

A tracked file gets git's skip-worktree bit; an untracked file is added to
.git/info/exclude. Both are local to the current work tree and are never
committed or pushed. Everything here shells out to the real `git` binary and
degrades to a no-op when git is absent or the path is outside a repository.
"""

import os
import subprocess


def _git(*args, check=False):
    """Run a git command; return the CompletedProcess, or None if git is absent."""
    try:
        return subprocess.run(
            ["git", *args], check=check, capture_output=True, text=True
        )
    except FileNotFoundError:
        return None


def git_tracked(path):
    """Return True if path is a file tracked by git in the current repo."""
    r = _git("ls-files", "--error-unmatch", "--", path)
    return r is not None and r.returncode == 0


def skip_worktree_set(path):
    """Return True if path currently carries git's skip-worktree bit."""
    r = _git("ls-files", "-v", "--", path)
    if r is None or r.returncode != 0:
        return False
    return r.stdout[:1] == "S"


def in_git_repo():
    """Return True if the current directory is inside a git work tree."""
    r = _git("rev-parse", "--is-inside-work-tree")
    return r is not None and r.returncode == 0 and r.stdout.strip() == "true"


def _git_path(name):
    """Path to a file inside the git dir (e.g. 'info/exclude'), or None."""
    r = _git("rev-parse", "--git-path", name)
    if r is None or r.returncode != 0:
        return None
    return r.stdout.strip()


def _exclude_pattern(path):
    """Anchored .gitignore pattern for path, relative to the repo top-level.

    Returns None if path lies outside the repository.
    """
    r = _git("rev-parse", "--show-toplevel")
    if r is None or r.returncode != 0:
        return None
    rel = os.path.relpath(os.path.abspath(path), r.stdout.strip())
    if rel.startswith(".."):
        return None
    return "/" + rel.replace(os.sep, "/")


def exclude_add(path):
    """Add path's anchored ignore pattern to .git/info/exclude.

    Returns True if the pattern is in place afterwards, False if it could not
    be applied (path outside the repo, or the exclude file unreachable).
    """
    pattern = _exclude_pattern(path)
    excl = _git_path("info/exclude")
    if pattern is None or excl is None:
        return False
    content = ""
    if os.path.exists(excl):
        with open(excl) as f:
            content = f.read()
    if pattern in content.splitlines():
        return True
    os.makedirs(os.path.dirname(excl), exist_ok=True)
    if content and not content.endswith("\n"):
        content += "\n"
    with open(excl, "w") as f:
        f.write(content + pattern + "\n")
    return True


def exclude_remove(path):
    """Remove path's anchored ignore pattern from .git/info/exclude.

    Returns True if a matching line was removed.
    """
    pattern = _exclude_pattern(path)
    excl = _git_path("info/exclude")
    if pattern is None or excl is None or not os.path.exists(excl):
        return False
    with open(excl) as f:
        lines = f.read().splitlines()
    kept = [ln for ln in lines if ln != pattern]
    if len(kept) == len(lines):
        return False
    with open(excl, "w") as f:
        f.write("\n".join(kept) + ("\n" if kept else ""))
    return True


def hide_in_worktree(path, prog, log=print):
    """Hide the workshop YAML from git so it never shows in 'git status'.

    A tracked file gets git's skip-worktree bit, hiding local edits. An
    untracked file is added to .git/info/exclude, keeping it off the untracked
    list. Both are local to this work tree and never committed or pushed. A
    silent no-op outside a git repository.
    """
    if not in_git_repo():
        return
    if git_tracked(path):
        if not skip_worktree_set(path):
            _git("update-index", "--skip-worktree", "--", path, check=True)
        log(
            f"Ignoring local changes to {path} in the work tree (git skip-worktree).\n"
            f"  'git status' won't show it as modified and 'git commit -a' skips it.\n"
            f"  To restore the tracked file or make a committable change: {prog} --revert"
        )
    elif exclude_add(path):
        log(
            f"Ignoring {path} in the work tree (.git/info/exclude).\n"
            f"  'git status' won't list it under untracked files. Local to this\n"
            f"  work tree only; never committed or pushed.\n"
            f"  To stop ignoring it: {prog} --revert"
        )


def revert(path, prog, log=print):
    """Undo hide_in_worktree.

    A tracked file has skip-worktree cleared and is restored to its git
    version. For an untracked file the local exclude entry is dropped; the
    file itself is left in place, since git never had a version to restore.
    """
    if not in_git_repo():
        log(f"{path} is not in a git repository; nothing to revert.")
        return
    if git_tracked(path):
        if skip_worktree_set(path):
            _git("update-index", "--no-skip-worktree", "--", path, check=True)
            log(f"Stopped ignoring {path} in the work tree (skip-worktree cleared).")
        _git("checkout", "--", path, check=True)
        log(f"Restored {path} to the version tracked in git.")
    elif exclude_remove(path):
        log(f"Stopped ignoring {path} in the work tree (.git/info/exclude cleared).")
    else:
        log(f"{path} is not ignored by {prog}; nothing to revert.")

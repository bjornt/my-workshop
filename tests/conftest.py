"""Pytest configuration.

Fixtures live in `tests/fixtures.py` and git helpers in `tests/git.py`; they are
re-exported here so pytest discovers the fixtures automatically.
"""

from tests.fixtures import git_repo, in_tmp

__all__ = ["git_repo", "in_tmp"]

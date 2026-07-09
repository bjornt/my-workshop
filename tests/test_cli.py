"""End-to-end tests for my_workshop.cli.

`main()` is driven against a real temporary git repo (the `git_repo` fixture,
which exercises the actual `git` binary) with an injected FakeWorkshop standing
in for the real `workshop` backend. Stdout is inspected via pytest's `capsys`.
No mocks: the only fake is the workshop backend that `main(workshop=...)`
exists to accept.
"""

import os

import pytest

from my_workshop.cli import build_parser, main
from tests.git import exclude_lines
from tests.fakes import FakeWorkshop


# --- build_parser -----------------------------------------------------------

def test_build_parser_maps_base_and_positional():
    # A non-default --base value must land on .base, and the positional on .yaml.
    args = build_parser().parse_args(["--base", "custom:img", "target.yaml"])
    assert args.base == "custom:img"
    assert args.yaml == "target.yaml"


def test_build_parser_revert_is_a_store_true_flag():
    # Presence of --revert flips it on; a store_false regression would fail here.
    assert build_parser().parse_args(["--revert", "target.yaml"]).revert is True


# --- main(): normal run -----------------------------------------------------

def test_main_fresh_repo_creates_hides_provisions_and_prints_hostname(
    git_repo, capsys
):
    fake = FakeWorkshop(hostname="dev-box")

    main(["workshop.yaml"], workshop=fake)

    # 1. The YAML is created on disk from the template.
    assert (git_repo / "workshop.yaml").exists()

    # 2. Freshly created and untracked -> hidden via an anchored .git/info/exclude
    #    entry so it drops off git's untracked list.
    assert "/workshop.yaml" in exclude_lines(git_repo)

    # 3. The full launch lifecycle ran, in order, before the hostname query.
    assert fake.ops[:5] == ["launch", "stop", "remount", "connect", "start"]

    # 4. main threaded its own omp_home through to the remount destination.
    assert fake.remounts == [("dev/omp:omp-home", os.path.expanduser("~/.omp"))]

    # 5. The connect hint prints the workshop's DNS hostname.
    assert "ssh workshop@dev-box" in capsys.readouterr().out


def test_main_falls_back_to_ip_when_no_hostname(git_repo, capsys):
    # No DNS hostname from `info` -> the printed hint uses the first `hostname -I`
    # address instead of a blank or a crash.
    main(["workshop.yaml"], workshop=FakeWorkshop(hostname=None))

    assert "ssh workshop@10.0.0.5" in capsys.readouterr().out


# --- main(): --revert short-circuits ----------------------------------------

def test_main_revert_does_not_provision(git_repo, capsys):
    # Establish real hidden state the way a prior normal run would.
    main(["workshop.yaml"], workshop=FakeWorkshop(hostname="dev-box"))
    assert "/workshop.yaml" in exclude_lines(git_repo)
    capsys.readouterr()  # discard the setup run's output

    fake = FakeWorkshop(hostname="dev-box")
    main(["workshop.yaml", "--revert"], workshop=fake)

    # The injected backend is never touched: no launch/stop/.../hostname query.
    assert fake.calls == []
    out = capsys.readouterr().out
    # ...and control returned before the connect hint is printed.
    assert "ssh workshop@" not in out
    # The revert branch actually ran against the resolved path: the file is
    # unhidden again.
    assert "/workshop.yaml" not in exclude_lines(git_repo)


# --- main(): explicit path --------------------------------------------------

def test_main_honours_explicit_path(git_repo):
    # An explicit positional path wins over the workshop.yaml default: the named
    # file is written and the default name is never created.
    main(["custom.yaml"], workshop=FakeWorkshop(hostname="dev-box"))

    assert (git_repo / "custom.yaml").exists()
    assert not (git_repo / "workshop.yaml").exists()

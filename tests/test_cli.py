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

def test_main_no_additions_is_noop_on_provision(git_repo, capsys):
    # Without an additions file the tool still creates and hides the YAML but
    # performs no copies or connections.
    fake = FakeWorkshop(hostname="dev-box")

    main(["workshop.yaml"], workshop=fake)

    # 1. The YAML is created on disk (minimal template, no SDKs).
    assert (git_repo / "workshop.yaml").exists()

    # 2. Hidden from git.
    assert "/workshop.yaml" in exclude_lines(git_repo)

    # 3. Provision runs launch/info/hostname but no copy or connect.
    assert fake.ops[:2] == ["launch", "info"]
    assert fake.copies == []
    assert fake.connections == []

    # 4. The log reports no additions config and still prints the connect hint.
    out = capsys.readouterr().out
    assert "No additions config found" in out
    assert "ssh workshop@" in out


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

    # The injected backend is never touched: no launch/.../hostname query.
    assert fake.calls == []
    out = capsys.readouterr().out
    # ...and control returned before the connect hint is printed.
    assert "ssh workshop@" not in out
    # The revert branch actually ran against the resolved path: the file is
    # unhidden again.
    assert "/workshop.yaml" not in exclude_lines(git_repo)

def test_main_revert_unhides_additions_file_too(git_repo, capsys):
    # Set up additions file and run once to hide both.
    (git_repo / "workshop.my.yaml").write_text("base: x\n")
    main(["workshop.yaml"], workshop=FakeWorkshop(hostname="dev-box"))
    assert "/workshop.my.yaml" in exclude_lines(git_repo)
    capsys.readouterr()

    main(["workshop.yaml", "--revert"], workshop=FakeWorkshop(hostname="dev-box"))

    assert "/workshop.yaml" not in exclude_lines(git_repo)
    assert "/workshop.my.yaml" not in exclude_lines(git_repo)


# --- main(): explicit path --------------------------------------------------

def test_main_honours_explicit_path(git_repo):
    # An explicit positional path wins over the workshop.yaml default: the named
    # file is written and the default name is never created.
    main(["custom.yaml"], workshop=FakeWorkshop(hostname="dev-box"))

    assert (git_repo / "custom.yaml").exists()
    assert not (git_repo / "workshop.yaml").exists()

# --- main(): local additions file ------------------------------------------

def test_main_with_local_additions_uses_custom_config(git_repo, capsys):
    # A local workshop.my.yaml with custom SDKs, base, and provision entries.
    additions = (
        "base: alpine@3.20\n"
        "sdks:\n"
        "  - name: custom-sdk\n"
        "    plugs:\n"
        "      my-plug:\n"
        "        interface: tunnel\n"
        "        endpoint: localhost:9000\n"
        "\n"
        "provision:\n"
        "  copy:\n"
        "    - source: ~/mydata\n"
        "      target: omp:omp-home\n"
        "  connect:\n"
        "    - plug: omp:pi-auth-gateway\n"
        "      slot: system:pi-auth-gateway\n"
    )
    (git_repo / "workshop.my.yaml").write_text(additions)

    fake = FakeWorkshop(hostname="dev-box")
    main(["workshop.yaml"], workshop=fake)

    out = capsys.readouterr().out
    assert "Using additions config" in out
    assert "workshop.my.yaml" in out

    # 1. Both YAML files are hidden from git.
    assert "/workshop.yaml" in exclude_lines(git_repo)
    assert "/workshop.my.yaml" in exclude_lines(git_repo)

    # 2. The workshop YAML was created with the custom SDKs.
    text = (git_repo / "workshop.yaml").read_text()
    assert "custom-sdk" in text

    # 3. The custom base was used.
    assert "base: alpine@3.20" in text
    # 4. The custom copy source was used.
    assert fake.copies == [
        (os.path.expanduser("~/mydata"), "/home/workshop/.omp"),
    ]
    # 5. The connect call uses the detected workshop name prefix.
    assert fake.connections == [
        ("dev/omp:pi-auth-gateway", "dev/system:pi-auth-gateway"),
    ]

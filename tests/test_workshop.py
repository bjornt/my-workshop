"""Tests for my_workshop.workshop orchestration, driven by FakeWorkshop.

All tests use FakeWorkshop (injected, not mocked) to verify the launch flow
without spawning real processes.
"""

import os

import pytest

from my_workshop.workshop import (
    hostname,
    parse_hostname,
    parse_mount_target,
    parse_workshop_name,
    provision,
)
from tests.fakes import FakeWorkshop


# A provision spec matching the built-in defaults (for tests that exercise the
# full flow without caring about custom entries).
DEFAULT_PROVISION = {
    "copy": [{"source": "~/.omp", "target": "omp:omp-home"}],
    "connect": [{"plug": "omp:pi-auth-gateway", "slot": "system:pi-auth-gateway"}],
}


# --- parse_hostname ---------------------------------------------------------

@pytest.mark.parametrize(
    "name, info_output, expected",
    [
        (
            "top_level_value",
            "name: dev\nbase: ubuntu@24.04\nhostname: dev-box\n",
            "dev-box",
        ),
        (
            "absent",
            "name: dev\nbase: ubuntu@24.04\n",
            None,
        ),
        (
            "indented_only_is_ignored",
            "name: dev\nsdks:\n  - name: try-omp\n    hostname: nope\n",
            None,
        ),
        (
            "indented_never_shadows_top_level",
            "    hostname: nope\nhostname: real-box\n",
            "real-box",
        ),
        (
            "blank_value",
            "name: dev\nhostname:\n",
            None,
        ),
        (
            "whitespace_only_value",
            "name: dev\nhostname:    \n",
            None,
        ),
        (
            "trailing_sdk_lines_ignored",
            "hostname: dev-box\nsdks:\n  - name: try-omp\n"
            "    hostname: indented-detail-should-be-ignored\n",
            "dev-box",
        ),
    ],
)
def test_parse_hostname(name, info_output, expected):
    assert parse_hostname(info_output) == expected


# --- parse_workshop_name ----------------------------------------------------

@pytest.mark.parametrize(
    "name, info_output, expected",
    [
        (
            "top_level_value",
            "name: dev\nbase: ubuntu@24.04\nhostname: dev-box\n",
            "dev",
        ),
        (
            "absent",
            "base: ubuntu@24.04\nhostname: dev-box\n",
            None,
        ),
        (
            "indented_name_ignored",
            "name: dev\nsdks:\n  - name: try-omp\n",
            "dev",
        ),
        (
            "indented_sdk_named_name_not_matched",
            "sdks:\n  - name: name\n",
            None,
        ),
        (
            "extra_whitespace",
            "name:   my-workshop  \n",
            "my-workshop",
        ),
    ],
)
def test_parse_workshop_name(name, info_output, expected):
    assert parse_workshop_name(info_output) == expected


# --- parse_mount_target ----------------------------------------------------

REAL_INFO = """\
name: dev
base: ubuntu@24.04
hostname: dev-box
sdks:
  omp:
    mounts:
      omp-home:
        workshop-target: /home/workshop/.omp
  zed-remote:
    mounts:
      zed-server:
        workshop-target: /home/workshop/.zed_server
"""


def test_parse_mount_target_extracts_workshop_target():
    assert parse_mount_target(REAL_INFO, "omp", "omp-home") == "/home/workshop/.omp"


def test_parse_mount_target_picks_correct_sdk_and_mount():
    assert parse_mount_target(REAL_INFO, "zed-remote", "zed-server") == "/home/workshop/.zed_server"


def test_parse_mount_target_returns_none_for_missing_sdk():
    assert parse_mount_target(REAL_INFO, "nope", "omp-home") is None


def test_parse_mount_target_returns_none_for_missing_mount():
    assert parse_mount_target(REAL_INFO, "omp", "nope") is None

# --- provision lifecycle ----------------------------------------------------

def test_provision_runs_lifecycle_ops_in_order():
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, DEFAULT_PROVISION)

    assert fake.ops[:4] == ["launch", "info", "copy_to", "connect"]


def test_provision_copies_each_spec():
    spec = {
        "copy": [
            {"source": "~/data", "target": "omp:omp-home"},
            {"source": "~/extra", "target": "zed-remote:zed-server"},
        ],
        "connect": [],
    }
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, spec)

    assert fake.copies == [
        (os.path.expanduser("~/data"), "/home/workshop/.omp"),
        (os.path.expanduser("~/extra"), "/home/workshop/.zed_server"),
    ]


def test_provision_connects_each_spec_with_workshop_prefix():
    spec = {
        "copy": [],
        "connect": [
            {"plug": "omp:pi-auth-gateway", "slot": "system:pi-auth-gateway"},
        ],
    }
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, spec)

    assert fake.connections == [
        ("dev/omp:pi-auth-gateway", "dev/system:pi-auth-gateway"),
    ]


def test_provision_autodetects_workshop_name():
    spec = {
        "copy": [],
        "connect": [
            {"plug": "omp:pi-auth-gateway", "slot": "system:pi-auth-gateway"},
        ],
    }
    fake = FakeWorkshop(hostname="dev-box", name="myws")

    provision(fake, spec)

    assert fake.connections == [
        ("myws/omp:pi-auth-gateway", "myws/system:pi-auth-gateway"),
    ]


# --- hostname resolution: DNS vs. IP fallback -------------------------------

def test_provision_returns_dns_hostname_and_skips_exec():
    fake = FakeWorkshop(hostname="dev-box")

    result = provision(fake, DEFAULT_PROVISION)

    assert result == "dev-box"
    # A DNS name from `info` short-circuits the fallback: no `exec` at all.
    assert "exec" not in fake.ops
    assert not any(c[0] == "exec" for c in fake.calls)


@pytest.mark.parametrize(
    "name, fake",
    [
        ("no_dns_hostname", FakeWorkshop(hostname=None)),
        ("info_command_fails", FakeWorkshop(info_ok=False)),
    ],
)
def test_hostname_falls_back_to_first_ip(name, fake):
    result = hostname(fake)

    # Falls back to the first token of `hostname -I` ("10.0.0.5 192.168.0.1 ").
    assert result == "10.0.0.5"
    # The fallback is what triggers the in-workshop exec query.
    assert ("exec", "hostname", "-I") in fake.calls


def test_provision_falls_back_to_ip_when_no_dns_hostname():
    fake = FakeWorkshop(hostname=None)

    result = provision(fake, DEFAULT_PROVISION)

    assert result == "10.0.0.5"
    assert "exec" in fake.ops

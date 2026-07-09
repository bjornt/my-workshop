"""Tests for my_workshop.workshop orchestration, driven by FakeWorkshop.

No real subprocess and no mocks: `provision`/`hostname` are exercised against
the in-memory FakeWorkshop, and `parse_hostname` against literal `info` text.
"""

import pytest

from my_workshop.workshop import (
    OMP_GATEWAY_PLUG,
    OMP_HOME_MOUNT,
    SYSTEM_GATEWAY_SLOT,
    hostname,
    parse_hostname,
    provision,
)
from tests.fakes import FakeWorkshop


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


# --- provision lifecycle ----------------------------------------------------

def test_provision_runs_lifecycle_ops_in_order():
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, "/home/dev/omp")

    # The five lifecycle ops fire in exactly this order (the trailing hostname
    # query -- info / exec -- is appended afterwards and checked separately).
    assert fake.ops[:5] == ["launch", "stop", "remount", "connect", "start"]


def test_provision_remounts_omp_home_at_the_configured_mount():
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, "/home/dev/omp")

    assert fake.remounts == [(OMP_HOME_MOUNT, "/home/dev/omp")]


def test_provision_connects_gateway_plug_to_system_slot():
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, "/home/dev/omp")

    assert fake.connections == [(OMP_GATEWAY_PLUG, SYSTEM_GATEWAY_SLOT)]


def test_provision_sets_lifecycle_flags():
    fake = FakeWorkshop(hostname="dev-box")

    provision(fake, "/home/dev/omp")

    assert (fake.launched, fake.stopped, fake.started) == (True, True, True)


# --- hostname resolution: DNS vs. IP fallback -------------------------------

def test_provision_returns_dns_hostname_and_skips_exec():
    fake = FakeWorkshop(hostname="dev-box")

    result = provision(fake, "/home/dev/omp")

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

    result = provision(fake, "/home/dev/omp")

    assert result == "10.0.0.5"
    assert "exec" in fake.ops

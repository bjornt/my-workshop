"""Tests for my_workshop.additions: config discovery, parsing, and defaults."""

import os

import pytest

from my_workshop.additions import (
    DEFAULT_ADDITIONS,
    find_additions,
    load_additions,
    parse_additions,
)


# --- DEFAULT_ADDITIONS shape ------------------------------------------------


def test_default_additions_is_empty():
    assert DEFAULT_ADDITIONS == {}


# --- parse_additions --------------------------------------------------------


FULL_CONFIG = (
    "base: alpine@3.20\n"
    "\n"
    "sdks:\n"
    "  - name: my-sdk\n"
    "    plugs:\n"
    "      my-plug:\n"
    "        interface: tunnel\n"
    "        endpoint: localhost:5000\n"
    "  - name: other-sdk\n"
    "    slots:\n"
    "      my-slot:\n"
    "        interface: tunnel\n"
    "        endpoint: localhost:6000\n"
    "\n"
    "provision:\n"
    "  copy:\n"
    "    - source: ~/data\n"
    "      target: my-sdk:data-mount\n"
    "    - source: ~/extra\n"
    "      target: other-sdk:extra-mount\n"
    "  connect:\n"
    "    - plug: my-sdk:my-plug\n"
    "      slot: other-sdk:my-slot\n"
)


def test_parse_additions_full_config():
    result = parse_additions(FULL_CONFIG)

    assert result["base"] == "alpine@3.20"

    assert len(result["sdks"]) == 2
    assert result["sdks"][0]["name"] == "my-sdk"
    assert result["sdks"][0]["plugs"]["my-plug"]["interface"] == "tunnel"
    assert result["sdks"][0]["plugs"]["my-plug"]["endpoint"] == "localhost:5000"
    assert result["sdks"][1]["name"] == "other-sdk"
    assert result["sdks"][1]["slots"]["my-slot"]["interface"] == "tunnel"

    assert len(result["provision"]["copy"]) == 2
    assert result["provision"]["copy"][0] == {"source": "~/data", "target": "my-sdk:data-mount"}
    assert result["provision"]["copy"][1] == {"source": "~/extra", "target": "other-sdk:extra-mount"}

    assert len(result["provision"]["connect"]) == 1
    assert result["provision"]["connect"][0] == {
        "plug": "my-sdk:my-plug",
        "slot": "other-sdk:my-slot",
    }


def test_parse_additions_sdks_only():
    text = (
        "sdks:\n"
        "  - name: alpha\n"
        "  - name: beta\n"
    )
    result = parse_additions(text)

    assert result["base"] is None
    assert [s["name"] for s in result["sdks"]] == ["alpha", "beta"]
    assert result["provision"]["copy"] == []
    assert result["provision"]["connect"] == []


def test_parse_additions_provision_only():
    text = (
        "provision:\n"
        "  copy:\n"
        "    - source: ~/x\n"
        "      target: sdk:mount\n"
    )
    result = parse_additions(text)

    assert result["base"] is None
    assert result["sdks"] == []
    assert result["provision"]["copy"] == [{"source": "~/x", "target": "sdk:mount"}]
    assert result["provision"]["connect"] == []


def test_parse_additions_base_only():
    text = "base: debian@12\n"
    result = parse_additions(text)

    assert result["base"] == "debian@12"
    assert result["sdks"] == []
    assert result["provision"]["copy"] == []


def test_parse_additions_empty_file():
    result = parse_additions("")

    assert result["base"] is None
    assert result["sdks"] == []
    assert result["provision"]["copy"] == []
    assert result["provision"]["connect"] == []


def test_parse_additions_whitespace_only():
    result = parse_additions("\n\n  \n")

    assert result["base"] is None
    assert result["sdks"] == []
    assert result["provision"]["copy"] == []


def test_parse_additions_malformed_ignored():
    # Random junk that isn't valid YAML structure should not crash.
    text = "not: valid: yaml: at all\n---\n  broken\n"
    result = parse_additions(text)
    assert result["base"] is None


# --- find_additions ---------------------------------------------------------


def test_find_additions_local_wins(in_tmp, monkeypatch):
    # Local file exists next to workshop YAML.
    workshop_path = str(in_tmp / "workshop.yaml")
    local_path = str(in_tmp / "workshop.my.yaml")
    with open(local_path, "w") as f:
        f.write("base: x\n")

    # Set HOME so a global could theoretically exist, but local should win.
    global_dir = in_tmp / ".config" / "my-workshop"
    global_dir.mkdir(parents=True)
    (global_dir / "my.yaml").write_text("base: y\n")
    monkeypatch.setenv("HOME", str(in_tmp))

    assert find_additions(workshop_path) == local_path


def test_find_additions_global_fallback(in_tmp, monkeypatch):
    workshop_path = str(in_tmp / "workshop.yaml")
    global_dir = in_tmp / ".config" / "my-workshop"
    global_dir.mkdir(parents=True)
    global_path = str(global_dir / "my.yaml")
    with open(global_path, "w") as f:
        f.write("base: z\n")
    monkeypatch.setenv("HOME", str(in_tmp))

    # No local file.
    assert find_additions(workshop_path) == global_path


def test_find_additions_neither_returns_none(in_tmp, monkeypatch):
    monkeypatch.setenv("HOME", str(in_tmp))
    assert find_additions(str(in_tmp / "workshop.yaml")) is None


def test_find_additions_local_in_subdir(in_tmp):
    # Workshop YAML is in a subdirectory; local file is alongside it.
    subdir = in_tmp / ".workshop"
    subdir.mkdir()
    workshop_path = str(subdir / "foo.yaml")
    local_path = str(subdir / "workshop.my.yaml")
    with open(local_path, "w") as f:
        f.write("base: x\n")

    assert find_additions(workshop_path) == local_path


# --- load_additions ---------------------------------------------------------


def test_load_additions_no_file_returns_empty(in_tmp, monkeypatch):
    monkeypatch.setenv("HOME", str(in_tmp))
    result = load_additions(str(in_tmp / "workshop.yaml"))

    assert result == {}


def test_load_additions_reads_local_file(in_tmp):
    workshop_path = str(in_tmp / "workshop.yaml")
    local_path = str(in_tmp / "workshop.my.yaml")
    with open(local_path, "w") as f:
        f.write("base: custom-img\nsdks:\n  - name: solo\n")

    result = load_additions(workshop_path)

    assert result["base"] == "custom-img"
    assert [s["name"] for s in result["sdks"]] == ["solo"]

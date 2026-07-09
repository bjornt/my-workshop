"""Tests for my_workshop.yaml_config: the pure line-editing of workshop YAML.

Every test names the observable contract it defends: the (name,start,end)
bounds iter_sdk_blocks reports, the append point sdks_end computes, the
sub-block/entry extraction find_subblock performs, the round-trip guarantee of
render_template, the in-place splice add_missing makes, the resolution order of
find_yaml, and the create/merge/idempotent behaviour of ensure_yaml.
"""

from my_workshop.yaml_config import (
    add_missing,
    ensure_yaml,
    find_subblock,
    find_yaml,
    iter_sdk_blocks,
    render_template,
    sdk_bounds,
    sdks_end,
)

import pytest

# Test fixture values matching the example additions config.
DEFAULT_BASE = "ubuntu@24.04"
GATEWAY = {"interface": "tunnel", "endpoint": "localhost:4000"}
REQUIRED_SDKS = [
    {"name": "try-zed-remote"},
    {"name": "try-omp", "plugs": {"pi-auth-gateway": GATEWAY}},
    {"name": "system", "slots": {"pi-auth-gateway": GATEWAY}},
]


def lines_of(text):
    return text.splitlines(keepends=True)


# A multi-SDK document whose last item (gamma) runs to EOF; alpha owns a
# plugs: sub-block, so its bounds must span past the nested entries.
MULTI_EOF = (
    "name: dev\n"
    "base: ubuntu@24.04\n"
    "sdks:\n"
    "  - name: alpha\n"
    "    plugs:\n"
    "      foo:\n"
    "        interface: tunnel\n"
    "  - name: beta\n"
    "  - name: gamma\n"
)

# Same shape but a trailing top-level key (extra:) terminates the sdks section,
# so the final SDK must NOT extend to EOF.
MULTI_TRAILING_KEY = (
    "name: dev\n"
    "base: ubuntu@24.04\n"
    "sdks:\n"
    "  - name: alpha\n"
    "  - name: beta\n"
    "    slots:\n"
    "      bar:\n"
    "        interface: tunnel\n"
    "extra: trailing\n"
)


# --- iter_sdk_blocks -------------------------------------------------------


def test_iter_sdk_blocks_last_item_ends_at_eof():
    lines = lines_of(MULTI_EOF)
    blocks = list(iter_sdk_blocks(lines))
    # alpha spans its nested plugs block (3..7), beta and gamma follow, and
    # gamma -- the last item -- ends one-past-last at len(lines).
    assert blocks == [("alpha", 3, 7), ("beta", 7, 8), ("gamma", 8, 9)]
    assert blocks[-1][2] == len(lines)


def test_iter_sdk_blocks_trailing_top_level_key_ends_section():
    lines = lines_of(MULTI_TRAILING_KEY)
    blocks = list(iter_sdk_blocks(lines))
    # beta absorbs its slots sub-block but stops at the `extra:` line (index 8),
    # not at EOF (index 9) -- the trailing top-level key closes the section.
    assert blocks == [("alpha", 3, 4), ("beta", 4, 8)]
    assert lines[blocks[-1][2]] == "extra: trailing\n"


# --- sdk_bounds ------------------------------------------------------------


def test_sdk_bounds_present_and_absent():
    lines = lines_of(MULTI_EOF)
    assert sdk_bounds(lines, "beta") == (7, 8)
    assert sdk_bounds(lines, "alpha") == (3, 7)
    assert sdk_bounds(lines, "nonexistent") is None


# --- sdks_end --------------------------------------------------------------


def test_sdks_end_stops_before_trailing_top_level_key():
    lines = lines_of(MULTI_TRAILING_KEY)
    # The append point is the index of the closing top-level key, so a new SDK
    # lands inside the sdks section rather than after `extra:`.
    end = sdks_end(lines)
    assert end == 8
    assert lines[end] == "extra: trailing\n"


def test_sdks_end_skips_trailing_blank_lines():
    lines = lines_of("name: dev\nsdks:\n  - name: alpha\n\n\n")
    # Two trailing blank lines are skipped: the insert point is right after the
    # last real SDK line, not at EOF.
    assert sdks_end(lines) == 3
    assert lines[sdks_end(lines) - 1] == "  - name: alpha\n"


# --- find_subblock ---------------------------------------------------------


SDK_WITH_PLUGS = (
    "  - name: try-omp\n"
    "    plugs:\n"
    "      pi-auth-gateway:\n"
    "        interface: tunnel\n"
    "      other-plug:\n"
    "        interface: tunnel\n"
)


def test_find_subblock_present_extracts_entry_names():
    lines = lines_of(SDK_WITH_PLUGS)
    header, entries = find_subblock(lines, 0, len(lines), "plugs")
    # Header at the indent-4 `plugs:` line; entry names taken from indent-6 keys.
    assert header == 1
    assert entries == {"pi-auth-gateway", "other-plug"}


def test_find_subblock_absent_returns_none_and_empty_set():
    lines = lines_of(SDK_WITH_PLUGS)
    # No slots: sub-block exists in this SDK.
    header, entries = find_subblock(lines, 0, len(lines), "slots")
    assert header is None
    assert entries == set()


# --- render_template -------------------------------------------------------


def test_render_template_round_trips_all_required_sdks():
    text = render_template(DEFAULT_BASE, REQUIRED_SDKS)
    assert text.startswith(f"name: dev\nbase: {DEFAULT_BASE}\nsdks:\n")
    names = [name for name, _, _ in iter_sdk_blocks(lines_of(text))]
    assert names == [spec["name"] for spec in REQUIRED_SDKS]


# --- add_missing -----------------------------------------------------------


# Three bare SDKs, none carrying its plug/slot sub-block yet.
BARE_SDKS = (
    "name: dev\n"
    "base: ubuntu@24.04\n"
    "sdks:\n"
    "  - name: try-zed-remote\n"
    "  - name: try-omp\n"
    "  - name: system\n"
)


def test_add_missing_splices_plug_into_sdk_lacking_subblock():
    lines = lines_of(BARE_SDKS)
    changed = add_missing(lines, "try-omp", "plugs", {"pi-auth-gateway": GATEWAY})
    assert changed is True

    # The plug is now discoverable in the try-omp block, with a fresh plugs:
    # header and the gateway attributes rendered at indent 6/8.
    bounds = sdk_bounds(lines, "try-omp")
    header, entries = find_subblock(lines, *bounds, "plugs")
    assert header is not None
    assert entries == {"pi-auth-gateway"}
    joined = "".join(lines)
    assert "    plugs:\n" in joined
    assert "      pi-auth-gateway:\n" in joined
    assert "        interface: tunnel\n" in joined
    assert "        endpoint: localhost:4000\n" in joined

    # Sibling SDKs are untouched and the section still parses to all three.
    assert [n for n, _, _ in iter_sdk_blocks(lines)] == [
        "try-zed-remote",
        "try-omp",
        "system",
    ]
    assert "  - name: try-zed-remote\n" in lines
    assert "  - name: system\n" in lines


def test_add_missing_absent_sdk_is_noop():
    lines = lines_of(BARE_SDKS)
    before = list(lines)
    assert add_missing(lines, "no-such-sdk", "plugs", {"x": GATEWAY}) is False
    assert lines == before


def test_add_missing_entry_already_present_is_noop():
    # try-omp already declares the gateway plug.
    text = (
        "name: dev\n"
        "sdks:\n"
        "  - name: try-omp\n"
        "    plugs:\n"
        "      pi-auth-gateway:\n"
        "        interface: tunnel\n"
    )
    lines = lines_of(text)
    before = list(lines)
    assert add_missing(lines, "try-omp", "plugs", {"pi-auth-gateway": GATEWAY}) is False
    assert lines == before


# --- find_yaml -------------------------------------------------------------


def test_find_yaml_honours_explicit_argument(in_tmp):
    # Explicit wins even when other candidates exist on disk.
    (in_tmp / "workshop.yaml").write_text("x")
    assert find_yaml("chosen.yaml") == "chosen.yaml"


def test_find_yaml_prefers_cwd_workshop_yaml(in_tmp):
    (in_tmp / "workshop.yaml").write_text("x")
    dot = in_tmp / ".workshop"
    dot.mkdir()
    (dot / "other.yaml").write_text("x")
    assert find_yaml(None) == "workshop.yaml"


def test_find_yaml_single_dotworkshop_candidate(in_tmp):
    dot = in_tmp / ".workshop"
    dot.mkdir()
    (dot / "only.yaml").write_text("x")
    import os

    assert find_yaml(None) == os.path.join(".workshop", "only.yaml")


def test_find_yaml_multiple_dotworkshop_candidates_raise(in_tmp):
    dot = in_tmp / ".workshop"
    dot.mkdir()
    (dot / "one.yaml").write_text("x")
    (dot / "two.yml").write_text("x")
    with pytest.raises(SystemExit):
        find_yaml(None)


def test_find_yaml_falls_back_when_nothing_exists(in_tmp):
    assert find_yaml(None) == "workshop.yaml"


# --- ensure_yaml -----------------------------------------------------------


def test_ensure_yaml_creates_from_template_and_logs(in_tmp):
    log = []
    ensure_yaml("workshop.yaml", DEFAULT_BASE, REQUIRED_SDKS, log=log.append)

    assert log == ["Created workshop.yaml"]
    text = (in_tmp / "workshop.yaml").read_text()
    # The created file is a valid template re-parseable to every required SDK.
    names = [name for name, _, _ in iter_sdk_blocks(lines_of(text))]
    assert names == [spec["name"] for spec in REQUIRED_SDKS]


def test_ensure_yaml_is_idempotent(in_tmp):
    ensure_yaml("workshop.yaml", DEFAULT_BASE, REQUIRED_SDKS, log=[].append)
    before = (in_tmp / "workshop.yaml").read_bytes()

    log = []
    ensure_yaml("workshop.yaml", DEFAULT_BASE, REQUIRED_SDKS, log=log.append)
    # Second run changes nothing and stays silent.
    assert log == []
    assert (in_tmp / "workshop.yaml").read_bytes() == before


def test_ensure_yaml_merges_missing_plug_preserving_lines(in_tmp):
    # A hand-written file: try-omp present but WITHOUT its gateway plug; a
    # comment and the fully-wired system slot are already there.
    hand = (
        "name: dev\n"
        "base: ubuntu@24.04\n"
        "# keep this comment\n"
        "sdks:\n"
        "  - name: try-zed-remote\n"
        "  - name: try-omp\n"
        "  - name: system\n"
        "    slots:\n"
        "      pi-auth-gateway:\n"
        "        interface: tunnel\n"
        "        endpoint: localhost:4000\n"
    )
    (in_tmp / "hand.yaml").write_text(hand)

    log = []
    ensure_yaml("hand.yaml", DEFAULT_BASE, REQUIRED_SDKS, log=log.append)

    assert log == ["Updated SDKs in hand.yaml: merged into try-omp (plugs)"]
    out = (in_tmp / "hand.yaml").read_text()
    # Existing lines survive verbatim.
    assert "# keep this comment\n" in out
    assert "  - name: try-zed-remote\n" in out
    # The missing plug got spliced into try-omp specifically.
    header, entries = find_subblock(
        lines_of(out), *sdk_bounds(lines_of(out), "try-omp"), "plugs"
    )
    assert header is not None
    assert entries == {"pi-auth-gateway"}
    # A re-run is now a silent no-op (nothing left to merge).
    log2 = []
    ensure_yaml("hand.yaml", DEFAULT_BASE, REQUIRED_SDKS, log=log2.append)
    assert log2 == []


def test_ensure_yaml_fully_specified_file_untouched(in_tmp):
    # A fully-specified, hand-placed file (the template itself, written to disk
    # directly rather than via ensure_yaml) must be left byte-for-byte alone.
    text = render_template(DEFAULT_BASE, REQUIRED_SDKS)
    (in_tmp / "full.yaml").write_text(text)
    before = (in_tmp / "full.yaml").read_bytes()

    log = []
    ensure_yaml("full.yaml", DEFAULT_BASE, REQUIRED_SDKS, log=log.append)
    assert log == []
    assert (in_tmp / "full.yaml").read_bytes() == before


def test_ensure_yaml_custom_sdks_merged(in_tmp):
    # A custom SDK list: only two SDKs, one with a plug.
    custom = [
        {"name": "alpha"},
        {"name": "beta", "plugs": {"my-plug": {"interface": "tunnel"}}},
    ]
    log = []
    ensure_yaml("workshop.yaml", DEFAULT_BASE, custom, log=log.append)

    assert log == ["Created workshop.yaml"]
    text = (in_tmp / "workshop.yaml").read_text()
    names = [name for name, _, _ in iter_sdk_blocks(lines_of(text))]
    assert names == ["alpha", "beta"]
    assert "my-plug" in text


def test_ensure_yaml_empty_sdks_creates_minimal_template(in_tmp):
    log = []
    ensure_yaml("workshop.yaml", DEFAULT_BASE, [], log=log.append)

    assert log == ["Created workshop.yaml"]
    text = (in_tmp / "workshop.yaml").read_text()
    assert "sdks:\n" in text
    # No SDK items — just the header.
    names = [name for name, _, _ in iter_sdk_blocks(lines_of(text))]
    assert names == []


def test_ensure_yaml_empty_sdks_noop_on_existing(in_tmp):
    # Pre-populate with a fully specified file.
    text = render_template(DEFAULT_BASE, REQUIRED_SDKS)
    (in_tmp / "workshop.yaml").write_text(text)
    before = (in_tmp / "workshop.yaml").read_bytes()

    log = []
    ensure_yaml("workshop.yaml", DEFAULT_BASE, [], log=log.append)
    assert log == []
    assert (in_tmp / "workshop.yaml").read_bytes() == before
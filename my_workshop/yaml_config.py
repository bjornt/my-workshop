"""Locate, create, and augment a workshop YAML file.

All editing is done on the raw lines of the file so that a hand-written YAML
keeps its comments, ordering, and formatting; only the missing SDKs and their
plugs/slots are spliced in.
"""

import glob
import os

DEFAULT_BASE = "ubuntu@24.04"

# Tunnel wiring shared by the try-omp plug and the system slot it connects to.
GATEWAY = {"interface": "tunnel", "endpoint": "localhost:4000"}

# Each required SDK and the plugs/slots it must provide. An SDK named here that
# is absent is injected whole; one that already exists (possibly declared for
# another purpose) has only its missing plugs/slots merged in.
REQUIRED_SDKS = [
    {"name": "try-zed-remote"},
    {"name": "try-omp", "plugs": {"pi-auth-gateway": GATEWAY}},
    {"name": "system", "slots": {"pi-auth-gateway": GATEWAY}},
]


def iter_sdk_blocks(lines):
    """Yield (name, start, end) for each SDK item under the sdks: block.

    start/end are line indices bounding the item, end being one past its last
    line (the next item, the next top-level key, or EOF).
    """
    in_sdks = False
    name = None
    start = None
    for i, raw in enumerate(lines):
        s = raw.rstrip()
        if not in_sdks:
            if s == "sdks:":
                in_sdks = True
            continue
        if s and not s.startswith(" "):   # next top-level key ends the section
            if start is not None:
                yield name, start, i
                start = None
            in_sdks = False
            continue
        stripped = s.strip()
        if s.startswith("  -") and stripped.startswith("- name:"):
            if start is not None:
                yield name, start, i
            name = stripped.split(":", 1)[1].strip()
            start = i
    if start is not None:
        yield name, start, len(lines)


def sdk_bounds(lines, name):
    """Return (start, end) for the SDK item with the given name, or None."""
    for n, start, end in iter_sdk_blocks(lines):
        if n == name:
            return start, end
    return None


def sdks_end(lines):
    """Return the index at which to append a new SDK item to the sdks: block."""
    end = len(lines)
    in_sdks = False
    for i, raw in enumerate(lines):
        s = raw.rstrip()
        if not in_sdks:
            if s == "sdks:":
                in_sdks = True
            continue
        if s and not s.startswith(" "):
            end = i
            break
    while end > 0 and not lines[end - 1].strip():   # skip trailing blank lines
        end -= 1
    return end


def find_subblock(lines, start, end, key):
    """Locate a 'plugs:'/'slots:' sub-block within an SDK item.

    Returns (header_index, entry_names); header_index is None if the sub-block
    is absent.
    """
    header = None
    entries = set()
    for i in range(start, end):
        s = lines[i].rstrip()
        if not s:
            continue
        indent = len(s) - len(s.lstrip())
        stripped = s.strip()
        if header is None:
            if indent == 4 and stripped == f"{key}:":
                header = i
            continue
        if indent <= 4:            # dedent to next key ends the sub-block
            break
        if indent == 6 and stripped.endswith(":"):
            entries.add(stripped[:-1])
    return header, entries


def render_entry(name, attrs, indent):
    pad = " " * indent
    out = [f"{pad}{name}:\n"]
    for k, v in attrs.items():
        out.append(f"{pad}  {k}: {v}\n")
    return "".join(out)


def render_sdk(spec):
    out = [f"  - name: {spec['name']}\n"]
    for kind in ("plugs", "slots"):
        entries = spec.get(kind)
        if entries:
            out.append(f"    {kind}:\n")
            for name, attrs in entries.items():
                out.append(render_entry(name, attrs, 6))
    return "".join(out)


def render_template(base):
    body = "".join(render_sdk(spec) for spec in REQUIRED_SDKS)
    return f"name: dev\nbase: {base}\nsdks:\n{body}"


def _ensure_newline(lines, i):
    if 0 <= i < len(lines) and not lines[i].endswith("\n"):
        lines[i] += "\n"


def add_missing(lines, name, kind, wanted):
    """Merge any missing `kind` (plugs/slots) entries into an existing SDK.

    Returns True if lines were modified.
    """
    if not wanted:
        return False
    bounds = sdk_bounds(lines, name)
    if bounds is None:
        return False
    start, end = bounds
    header, existing = find_subblock(lines, start, end, kind)
    missing = [(k, v) for k, v in wanted.items() if k not in existing]
    if not missing:
        return False
    blob = "".join(render_entry(k, v, 6) for k, v in missing)
    if header is None:
        last = start
        for i in range(start, end):
            if lines[i].strip():
                last = i
        _ensure_newline(lines, last)
        insert_at = last + 1
        blob = f"    {kind}:\n" + blob
    else:
        insert_at = header + 1
    lines[insert_at:insert_at] = blob.splitlines(keepends=True)
    return True


def find_yaml(explicit):
    """Locate the workshop YAML file to use.

    Preference order: an explicitly given path, then workshop.yaml, then a
    single *.yaml/*.yml file under .workshop/. Multiple candidates under
    .workshop/ are ambiguous and require an explicit path. The returned file
    need not exist yet (it will be created from the template).
    """
    if explicit:
        return explicit

    if os.path.exists("workshop.yaml"):
        return "workshop.yaml"

    candidates = sorted(
        glob.glob(os.path.join(".workshop", "*.yaml"))
        + glob.glob(os.path.join(".workshop", "*.yml"))
    )
    if len(candidates) > 1:
        raise SystemExit(
            "Multiple YAML files found in .workshop/: "
            + ", ".join(candidates)
            + "\nPass the path explicitly."
        )
    if candidates:
        return candidates[0]

    return "workshop.yaml"


def ensure_yaml(path, base, log=print):
    """Create `path` from the template, or merge in any missing required SDKs.

    Prints a short summary of what changed via `log`. A no-op (no print) when
    the file already declares every required SDK and plug/slot.
    """
    if not os.path.exists(path):
        with open(path, "w") as f:
            f.write(render_template(base))
        log(f"Created {path}")
        return

    with open(path) as f:
        lines = f.readlines()

    present = {name for name, _, _ in iter_sdk_blocks(lines)}
    added = []
    merged = []

    for spec in REQUIRED_SDKS:
        name = spec["name"]
        if name not in present:
            insert_at = sdks_end(lines)
            _ensure_newline(lines, insert_at - 1)
            lines[insert_at:insert_at] = render_sdk(spec).splitlines(keepends=True)
            added.append(name)
        else:
            fields = [
                kind for kind in ("plugs", "slots")
                if add_missing(lines, name, kind, spec.get(kind, {}))
            ]
            if fields:
                merged.append(f"{name} ({'+'.join(fields)})")

    if not added and not merged:
        return

    with open(path, "w") as f:
        f.writelines(lines)

    parts = []
    if added:
        parts.append("added " + ", ".join(added))
    if merged:
        parts.append("merged into " + ", ".join(merged))
    log(f"Updated SDKs in {path}: " + "; ".join(parts))

"""Load external additions config for my-workshop.

The additions file lets users configure the SDK list, base image,
copy targets, and plug/slot connections.  Two locations are searched in order:

1. **Local** – ``workshop.my.yaml`` next to the resolved workshop YAML path.
2. **Global** – ``~/.config/my-workshop/my.yaml``.

When neither file exists, an empty dict is returned (noop).
See ``workshop.my.yaml.example`` for the full config format.
"""

import os

from .yaml_config import find_subblock, iter_sdk_blocks

# No built-in defaults — when no additions file is found the tool is a noop.
# Copy workshop.my.yaml.example and customise it for your project.
DEFAULT_ADDITIONS = {}


# ---------------------------------------------------------------------------
# File discovery
# ---------------------------------------------------------------------------

def find_additions(workshop_yaml_path):
    """Return the path to the additions file, or ``None``.

    *workshop_yaml_path* is the resolved path returned by ``find_yaml``.
    """
    # Local: same directory as the workshop YAML.
    local = os.path.join(os.path.dirname(workshop_yaml_path), "workshop.my.yaml")
    if os.path.exists(local):
        return local

    # Global: ~/.config/my-workshop/my.yaml
    home = os.environ.get("HOME", "")
    if home:
        global_path = os.path.join(home, ".config", "my-workshop", "my.yaml")
        if os.path.exists(global_path):
            return global_path

    return None


# ---------------------------------------------------------------------------
# Parsing
# ---------------------------------------------------------------------------

def _read_scalar(lines, key):
    """Return the value of a top-level ``key: value`` line, or ``None``."""
    prefix = key + ":"
    for line in lines:
        s = line.rstrip()
        if not s:
            continue
        if s.lstrip() == s and s.startswith(prefix):
            val = s[len(prefix):].strip()
            return val or None
    return None


def _subblock_range(lines, start, end, key):
    """Find ``(sub_start, sub_end)`` for ``key:`` between *start* and *end*.

    Returns ``None`` when *key* is not found.  ``sub_start`` is the line
    *after* the ``key:`` header; ``sub_end`` is one past the last line that
    belongs to the block (blank lines inside the block are included).
    """
    for i in range(start, end):
        s = lines[i].rstrip()
        if not s:
            continue
        indent = len(s) - len(s.lstrip())
        stripped = s.strip()
        if stripped == f"{key}:":
            sub_start = i + 1
            # Collect subsequent lines that are indented deeper than *key*.
            sub_end = sub_start
            for j in range(sub_start, end):
                sj = lines[j].rstrip()
                if not sj:
                    sub_end = j + 1
                    continue
                if len(sj) - len(sj.lstrip()) > indent:
                    sub_end = j + 1
                else:
                    break
            return sub_start, sub_end
    return None


def _parse_subblock_attrs(lines, header_idx):
    """Parse ``key: value`` pairs indented under the line at *header_idx*.

    Returns a dict of attributes.  The header line itself (e.g.
    ``my-plug:``) is at some indent; its children are one level deeper.
    """
    attrs = {}
    s = lines[header_idx].rstrip()
    base_indent = len(s) - len(s.lstrip())
    for j in range(header_idx + 1, len(lines)):
        sj = lines[j].rstrip()
        if not sj:
            continue
        indent = len(sj) - len(sj.lstrip())
        if indent <= base_indent:
            break
        # Only take direct children (one level deeper).
        if indent == base_indent + 2 and ":" in sj:
            k, _, v = sj.strip().partition(":")
            k = k.strip()
            v = v.strip()
            if k and v:
                attrs[k] = v
    return attrs


def _parse_list_items(lines, start, end, fields):
    """Yield dicts for YAML list items between *start* and *end*.

    *fields* is the set of scalar field names to extract (e.g.
    ``{"source", "target"}``).  Items are detected by ``- key: value`` lines
    at the first list-indent level, with subsequent ``key: value`` lines at
    the next level.
    """
    # Detect the base indent from the first non-empty line.
    base_indent = None
    for i in range(start, end):
        s = lines[i].rstrip()
        if s:
            base_indent = len(s) - len(s.lstrip())
            break
    if base_indent is None:
        return

    list_indent = base_indent          # indent of ``- ...``
    field_indent = base_indent + 2     # indent of subsequent ``key: val``
    item = None

    for i in range(start, end):
        s = lines[i].rstrip()
        if not s:
            continue
        indent = len(s) - len(s.lstrip())
        stripped = s.strip()

        if indent == list_indent and stripped.startswith("- "):
            # New list item.  Yield the previous one first.
            if item is not None:
                yield item
            item = {}
            # First field may sit on the same line as the dash.
            rest = stripped[2:].strip()
            if ":" in rest:
                k, _, v = rest.partition(":")
                k, v = k.strip(), v.strip()
                if k in fields and v:
                    item[k] = v
        elif indent == field_indent and item is not None and ":" in stripped:
            k, _, v = stripped.partition(":")
            k, v = k.strip(), v.strip()
            if k in fields and v:
                item[k] = v

    if item is not None:
        yield item


def parse_additions(text):
    """Parse additions config text into the standard dict shape.

    Returns ``{"base": str|None, "sdks": [...], "provision": {...}}``.
    Missing sections are filled with empty defaults.
    """
    lines = text.splitlines(keepends=True)
    result = {"base": None, "sdks": [], "provision": {"copy": [], "connect": []}}

    # --- base ---
    result["base"] = _read_scalar(lines, "base")

    # --- sdks ---  (reuse existing iter_sdk_blocks / find_subblock)
    for name, start, end in iter_sdk_blocks(lines):
        spec = {"name": name}
        for kind in ("plugs", "slots"):
            header, entries = find_subblock(lines, start, end, kind)
            if header is not None:
                block = {}
                # Find the range of the plugs/slots sub-block.
                bounds = _subblock_range(lines, start, end, kind)
                if bounds is not None:
                    sub_start, sub_end = bounds
                    for entry_name in entries:
                        # Find the entry's header line within the sub-block.
                        for i in range(sub_start, sub_end):
                            stripped = lines[i].rstrip().strip()
                            if stripped == f"{entry_name}:":
                                block[entry_name] = _parse_subblock_attrs(lines, i)
                                break
                spec[kind] = block
        result["sdks"].append(spec)

    # --- provision ---
    prov = _subblock_range(lines, 0, len(lines), "provision")
    if prov is not None:
        ps, pe = prov
        copy = _subblock_range(lines, ps, pe, "copy")
        if copy is not None:
            result["provision"]["copy"] = list(
                _parse_list_items(lines, copy[0], copy[1], {"source", "target"})
            )
        connect = _subblock_range(lines, ps, pe, "connect")
        if connect is not None:
            result["provision"]["connect"] = list(
                _parse_list_items(lines, connect[0], connect[1], {"plug", "slot"})
            )

    return result


# ---------------------------------------------------------------------------
# Loading
# ---------------------------------------------------------------------------

def load_additions(workshop_yaml_path):
    """Load the additions config, returning ``{}`` when no file is found."""
    path = find_additions(workshop_yaml_path)
    if path is None:
        return {}
    with open(path) as f:
        text = f.read()
    return parse_additions(text)

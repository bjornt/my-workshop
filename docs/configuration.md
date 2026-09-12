# Configuration reference

[Documentation index](README.md) · [Usage and runtime behavior](usage.md)

`my-workshop` reads an optional **additions file**, then adds missing SDKs,
plugs, and slots to the selected workshop YAML. The additions file is not a
replacement workshop definition: its `provision` section drives actions after
launch rather than being written into the workshop YAML.

## Which additions file is used?

The CLI first selects the workshop YAML ([path selection](usage.md)), then
looks for additions in this order:

1. `workshop.my.yaml` in the selected YAML's directory.
2. `$HOME/.config/my-workshop/my.yaml`, if `HOME` is nonempty.

Only one file is read. **Local and global configuration are not merged.** An
empty local file also takes precedence over the global file. The global path
is fixed; `XDG_CONFIG_HOME` is not consulted.

For example, selecting `.workshop/dev.yaml` looks for
`.workshop/workshop.my.yaml`, not `./workshop.my.yaml`. If both files exist
under `.workshop/`, pass the workshop path explicitly:

```sh
my-workshop .workshop/dev.yaml
```

Auto-detection considers every `.yaml` and `.yml` file in `.workshop/`, including
the additions file, so this layout otherwise produces multiple candidates.

A candidate is selected when its filesystem stat succeeds. A stat failure
causes lookup to try the next location. If reading the selected candidate
fails, the loader silently uses empty additions; it does **not** retry the
global file. The CLI may still print `Using additions config: ...` because that
message reflects discovery, not successful reading. Empty files and omitted
sections also yield empty additions. There is no configuration parse-error
reporting or schema validation.

**No additions does not mean no side effects.** Despite the `running as noop`
message, a normal invocation still creates a missing workshop YAML, attempts
to hide it from Git status, launches Workshop,
and obtains connection information. It adds no SDKs and performs no configured
copies or connections. See [usage](usage.md) for Git hiding, revert, and failure
behavior.

## Base image precedence

For a **new** workshop YAML, the first nonempty value wins:

1. CLI `--base IMAGE`.
2. `base:` in the selected additions file.
3. `ubuntu@24.04`.

The generated file starts with `name: dev`, the selected `base`, and `sdks:`.
With no configured SDKs, `sdks:` has no list items. Parent directories are not
created automatically. An existing workshop YAML keeps its base unchanged,
even if `--base` or the additions file specifies another value; a missing base
in an existing file is not filled in either.

## Supported fields and example

All sections are optional. This example follows
[`workshop.my.yaml.example`](../workshop.my.yaml.example):

```yaml
base: ubuntu@24.04

sdks:
  - name: try-zed-remote
  - name: try-omp
    plugs:
      pi-auth-gateway:
        interface: tunnel
        endpoint: localhost:4000
  - name: system
    slots:
      pi-auth-gateway:
        interface: tunnel
        endpoint: localhost:4000

provision:
  copy:
    - source: ~/.omp
      target: omp:omp-home
  connect:
    - plug: omp:pi-auth-gateway
      slot: system:pi-auth-gateway
```

| Field | Current meaning |
| --- | --- |
| `base` | Base image text for creation only, subject to the precedence above. |
| `sdks` | Ordered list of SDK definitions to add or extend. There is no built-in SDK list. |
| `sdks[].name` | SDK name used for exact-text matching against the workshop YAML. |
| `sdks[].plugs` / `sdks[].slots` | Named entries, each containing flat `key: value` attributes. `interface` and `endpoint` are example attributes, not a fixed allowlist. |
| `provision.copy[].source` | Host directory whose contents will be copied. Leading `~` or `~/` expands to the user's home; other relative paths are relative to the invocation directory, not the additions file. |
| `provision.copy[].target` | `sdk:mount` identifier, resolved to a destination using `workshop info`; not a destination filesystem path. |
| `provision.connect[].plug` | `sdk:plug` identifier. The running workshop name is prepended before connecting. |
| `provision.connect[].slot` | `sdk:slot` identifier, with the same workshop-name prefix. |

SDKs, entries, attributes, and provisioning items retain their input order when
parsed. Other SDK-level fields are not imported from the additions file; copy
items only recognize `source` and `target`, and connect items only recognize
`plug` and `slot`. Incomplete items are not rejected: omitted fields become
empty strings and reach the runtime. Supply both fields for each action.

The example's SDK names, mounts, and tunnel endpoints are environment-specific,
not bundled or verified by `my-workshop`. In particular, its SDK definition is
named `try-omp`, while provisioning refers to `omp`; the wrapper does not
translate or validate that relationship. Check the names and mount exposed by
your Workshop installation before using it. The name `pi-auth-gateway` is just
an entry name, not a guarantee of an authentication service or secure credential
handling. Copying `~/.omp` transfers that directory's contents, potentially
including credentials; copied files can overwrite destination files. Read
[copy and connect execution](usage.md) before enabling these actions.

## Syntax: a constrained, line-oriented format

The implementation scans lines rather than using a general YAML parser. Use
the block layout and space indentation above:

- `base:` and the exact `sdks:` header start at column one. SDK items start with
  two spaces followed by `- name:`; `name` must be the first item field.
- `plugs:` and `slots:` use four spaces, entry names six, and attributes eight.
  Attributes must be nonempty, single-line `key: value` pairs.
- Use `provision:` at column one, `copy:` / `connect:` at two spaces, list items
  at four spaces, and subsequent item fields at six spaces.
- Keep headers on their own lines. Do not use tabs, inline lists or maps,
  anchors, aliases, merge keys, or multiline scalar syntax.
- Values are trimmed text, not decoded YAML scalars: quotes, escapes, and inline
  comments are not interpreted. In particular, quoted paths or runtime
  identifiers retain their quote characters. Prefer plain, unquoted values
  without inline comments.
- Comments are not understood structurally. A column-one comment inside an SDK
  list ends that list; comments or whitespace-only lines within nested blocks
  can interrupt recognition or be mistaken for attributes. Keep explanatory
  comments outside the blocks and use truly empty separator lines.
- Avoid duplicate SDK and entry names. They are not validated or reliably
  deduplicated. Unsupported or malformed structures may be ignored or
  partially interpreted rather than producing an error.

These restrictions also matter in an **existing workshop YAML**, because the
same SDK scanner is used for merging. Valid general YAML is not necessarily
recognized by this tool.

## How the workshop YAML is changed

The merge is additive and name-based:

- A missing SDK is inserted at the end of the recognized `sdks:` section,
  before a following top-level line and trailing blank lines.
- For an existing SDK, only missing named plugs and slots are inserted. If the
  relevant section exists, new entries go immediately after its header; if it
  does not, a section is added at the end of the SDK block.
- **Existing entries win in full.** An existing plug or slot is not updated,
  even if its attributes differ or an attribute is missing. For example, an
  existing `pi-auth-gateway` with only `interface: tunnel` will not gain the
  example's `endpoint` attribute.
- Removing or changing additions does not remove previously inserted SDKs or
  entries. Existing fields outside these insertions are not rewritten.

When nothing is missing, the file is not written and its bytes stay unchanged.
When additions are inserted, existing lines are retained rather than
reformatted, including comments the scanner can work around. New lines use the
fixed indentation above and LF newlines; an insertion may add a missing final
newline. This is not a general YAML round-trip or repair mechanism.

In particular, an existing file must already contain a recognizable `sdks:`
header before SDKs can be safely added. The merge does not create that header
in an existing file: if it is absent or written in another form such as
`sdks: []`, SDK lines are appended without a new header and can leave the file
invalid. Keep the supported layout and review the YAML directly, since Git
hiding can conceal these edits from ordinary status and diff output.

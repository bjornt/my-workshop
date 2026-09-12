# Documentation

`my-workshop` adds personal configuration to Workshop and launches it. Start
with the [repository README](../README.md) for installation and a first run.
These pages describe the implementation in this repository, including its
current limitations.

## Using my-workshop

| Page | Purpose |
| --- | --- |
| [Usage and recovery](usage.md) | How to run the CLI, select a YAML file, understand provisioning, and manage local Git hiding. |
| [Configuration reference](configuration.md) | Additions lookup, supported syntax and fields, base selection, and additive YAML editing. |

## Developing my-workshop

| Page | Purpose |
| --- | --- |
| [Development guide](development.md) | Package boundaries, commands, testing seams, and proposing and delivering features. |
| [Vision](VISION.md) | Stable product promise, desired experience, principles, and boundaries. This is direction, not current-feature documentation. |

## Maintaining this documentation

Keep this small, flat set organized by purpose rather than by individual feature:

- Installation and first-run instructions belong in the repository README.
- User procedures, CLI behavior, failure states, and recovery belong in
  `usage.md`.
- Configuration syntax, precedence, and merge rules belong in
  `configuration.md`.
- Contributor instructions and the current code structure belong in
  `development.md`.
- Strategic direction belongs in `VISION.md`; change it only after an explicit
  product decision, not to justify an implementation after the fact.

Update the existing owning page first. Add a page only when no existing page
can usefully cover the subject, and link it here. User-facing behavior is
written once in the user pages and linked from contributor material.

The [Change Compass skills](../.agents/skills/README.md) use this index to find
durable documentation. Active `SPEC.md`, `PLAN.md`, and `EPIC.md` files are
temporary planning artifacts, not a second documentation set. Durable
architectural decisions belong in `docs/adr/` when needed, using the skills'
ADR format. No ADRs are recorded yet; this documentation baseline describes
existing structure without inventing historical decisions.

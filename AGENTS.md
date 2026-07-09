# AGENTS.md

Guidance for AI coding agents working in this repository.

## Project

`my-workshop` bootstraps, git-ignores, and launches a `workshop` development
environment. See `README.md` for user-facing purpose and usage. The logic lives
in the importable `my_workshop` package; the `my-workshop` file at the repo root
is a thin entrypoint shim that delegates to `my_workshop.cli:main`.

## Environment & commands

The project is managed with [`uv`](https://docs.astral.sh/uv/). `uv` installs to
`~/.local/bin`; if it is not on `PATH`, prefix commands with
`export PATH="$HOME/.local/bin:$PATH"`.

| Task                     | Command                          |
| ------------------------ | -------------------------------- |
| Install / sync env       | `uv sync`                        |
| Run the whole test suite | `uv run pytest`                  |
| Run one test file        | `uv run pytest tests/test_cli.py`|
| Run the entrypoint       | `uv run my-workshop --help`      |

Do not `pip install` into the system Python or hand-edit `.venv`; go through
`uv`. `.venv/`, `__pycache__/`, and `.pytest_cache/` are gitignored — never
commit them.

## Architecture

Each module owns one concern; keep them separated. Do not fold subprocess calls
or git side effects back into the pure logic.

| Module                       | Responsibility                                          |
| ---------------------------- | ------------------------------------------------------- |
| `my_workshop/yaml_config.py` | Pure line-editing of the workshop YAML. No I/O beyond reading/writing the target file; no subprocess. |
| `my_workshop/additions.py`  | Load/parse the external additions config (`workshop.my.yaml` next to the workshop YAML, or `~/.config/my-workshop/my.yaml`). Pure parsing; file I/O only in `load_additions`. Returns `{}` (noop) when no file is found. |
| `my_workshop/worktree.py`    | Hide/reveal the YAML from git. Shells out to the real `git` binary; degrades to a no-op when git is absent or the path is outside a repo. |
| `my_workshop/workshop.py`    | `Workshop` wraps the `workshop` CLI; `provision(ws, provision_spec)`/`hostname(ws)` orchestrate against any Workshop-shaped object. `parse_hostname` and `parse_workshop_name` are pure. |
| `my_workshop/cli.py`         | `argparse` + `main(argv=None, workshop=None)`.          |

### Seams — preserve them

These injection points exist so the code is testable without mocks. Keep them
when you extend the code:

- Side-effecting functions (`ensure_yaml`, `hide_in_worktree`, `revert`) take a
  `log=print` parameter instead of calling `print` directly — tests pass
  `log=list.append` to capture output without touching stdout.
- `main(argv=None, workshop=None)` accepts a parsed-args override and an
  injected workshop backend. `provision(ws, provision_spec)` / `hostname(ws)`
  take the workshop object as an argument rather than constructing the real one.

If you add a new external dependency (another CLI, network call, etc.), add a
similar seam and a fake — do not reach for `subprocess` inline in logic that
tests will need to exercise.

## Testing

Tests live in `tests/`. The overriding rule:

> **No mocks.** Do not use `unittest.mock`, and do not `monkeypatch.setattr`
> our own functions to stub behaviour. `monkeypatch.chdir` / `monkeypatch.setenv`
> (cwd and environment only) are fine.

Instead:

- **Fakes for external tools.** Anything that talks to the `workshop` CLI is
  driven by `FakeWorkshop` in `tests/fakes.py`, injected via
  `main(workshop=...)` / `provision(fake, ...)`. When you add a new external
  integration, follow this pattern: build a fake that simulates the observable
  behaviour, don't mock the call.
- **Real git for worktree behaviour.** Use the `git_repo` fixture in
  `tests/conftest.py` (a real temporary repo) so tests verify actual
  `git status` / `skip-worktree` / `.git/info/exclude` effects.
- Shared fixtures/helpers live in `tests/conftest.py`; the fake in
  `tests/fakes.py`. Reuse them — don't re-declare them per file.

Assert observable behaviour and invariants, not implementation details or
default values. Every behavioural change must ship with a test that would fail
without it. Run `uv run pytest` before finishing.

## Conventions

- Match the existing style: 4-space indent, module docstrings, plain-stdlib
  Python (no third-party runtime dependencies — `dependencies = []` in
  `pyproject.toml`; keep it that way unless there is a strong reason).
- Prefer editing an existing module over adding a new one. Remove obsolete code
  rather than leaving shims or dead branches.
- Don't create documentation files unless asked.

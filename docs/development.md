# Development guide

[Documentation index](README.md) · [User behavior](usage.md) ·
[Configuration reference](configuration.md)

## Build and verify

The module is `github.com/bjornt/my-workshop`, targets Go 1.23, and has no
third-party Go dependencies. Use the standard library unless a new dependency
has a strong justification.

From the repository root:

```sh
make build              # go build -o my-workshop ./cmd/my-workshop
make test               # go test ./...
make fmt                # gofmt -w .
make vet                # go vet ./...
make check              # format, vet, and test
```

`make check` rewrites Go formatting; it does not include a build. CI separately
checks formatting, vets, builds all packages, and runs the suite with Go 1.23.
See the [Makefile](../Makefile) and [CI workflow](../.github/workflows/tests.yml).

For focused work and a side-effect-free CLI help check:

```sh
go test ./internal/cli/...
go run ./cmd/my-workshop --help
```

Tests use a fake Workshop backend and real temporary Git repositories. Workshop
is not needed to run the suite; install Git to exercise the worktree tests.
A real launch requires the [runtime prerequisites](../README.md#requirements).
Do not use the normal launch command in this checkout merely to test help: it
can edit and hide configuration, launch Workshop, and provision host data.

## Current structure

| Package | Responsibility |
| --- | --- |
| [`cmd/my-workshop`](../cmd/my-workshop/main.go) | Thin entrypoint: supplies the logger and real backend selection, prints errors to stderr, and exits nonzero on failure. |
| [`internal/cli`](../internal/cli/cli.go) | Parses flags and orchestrates YAML discovery, additions loading, revert or setup, Git hiding, and provisioning. |
| [`internal/additions`](../internal/additions/additions.go) | Discovers, reads, and parses the optional additions file into SDK and provisioning specifications. |
| [`internal/yamlconfig`](../internal/yamlconfig/yamlconfig.go) | Discovers the Workshop YAML and performs line-oriented rendering and additive editing. No subprocess calls. |
| [`internal/worktree`](../internal/worktree/worktree.go) | Uses the real Git binary for tracked-file flags and local excludes, with best-effort behavior when Git is unavailable. |
| [`internal/workshop`](../internal/workshop/workshop.go) | Defines the external-tool interface, provisioning orchestration, info parsers, and the `os/exec` backend for Workshop and tar. |
| [`internal/testsupport`](../internal/testsupport/) | Shared fake Workshop backend and real-Git/scratch-directory helpers. |

These are internal packages, not a public library API. Keep parsing and editing
separate from subprocess and Git effects. The configuration code is deliberately
line-oriented; do not assume it accepts general YAML. Its supported shapes and
preservation limits are documented in the [configuration reference](configuration.md).

## Testing seams and conventions

- `cli.Run(argv, ws, log)` accepts an injected `workshop.Workshop`. Passing `nil`
  selects the real backend.
- `workshop.Provision(ws, spec)` and `workshop.Hostname(ws)` accept a backend
  rather than constructing one internally. The interface covers launch,
  directory copy, connection, info, and command execution.
- Side-effecting paths accept `Logger` functions (`func(string)`). Tests capture
  their output with a closure rather than redirecting process stdout. CLI help
  is printed directly by flag parsing.
- Use [`fakeworkshop.New`](../internal/testsupport/fakeworkshop/fakeworkshop.go)
  for the external Workshop seam. It supplies info/hostname data and records
  provisioning operations; it does not launch a container or transfer files.
- Use [`gitenv.NewRepo`](../internal/testsupport/gitenv/gitenv.go) to verify actual
  Git behavior and `gitenv.NewTmp` for scratch working directories. The helpers
  restore the original directory during cleanup. They change process-wide state:
  do not parallelize tests that use them. Do not use `t.Chdir`, which requires
  Go 1.24.

Tests live beside the implementation in `*_test.go`. Assert observable behavior,
errors, boundaries, and recovery rather than implementation details. Do not mock
our own functions. New external integrations should have an explicit seam and a
behavioral fake, not inline subprocess calls inside otherwise pure logic.

Every behavioral change needs a regression test that would fail without it.
Use the relevant real-surface smoke check as well: a fake-backed orchestration
test does not verify compatibility with the installed Workshop CLI. Run
`go test ./...` before finishing. For documentation-only work, validate links and
examples against the current code and command output; do not add tests of prose.

## Defining and delivering features

The repository includes [Change Compass](../.agents/skills/README.md): six agent
skills backed by ordinary Markdown. Start with the [vision](VISION.md) and the
current user documentation. The vision is a strategic alignment point, not a
backlog or evidence that a capability already exists.

For one independently deliverable change:

```text
/skill:change-propose <change-name>
/skill:change-implement <change-name>
/skill:change-finish <change-name>
```

Run these as separate stages: review the proposal before implementation.
Proposal creates `changes/<change-name>/SPEC.md` (desired experience and scope)
and `PLAN.md` (approach, risks, and verifiable tasks). Implementation follows the
plan and updates the owning documentation pages. Finish independently audits
the delivered result, reconciles durable knowledge, and removes the temporary
change directory. Git retains history.

For one outcome requiring several independently deliverable changes:

```text
/skill:epic-propose <epic-name>
/skill:epic-propose-next <epic-name>
/skill:change-implement <epic-name>/<change-name>
/skill:change-finish <epic-name>/<change-name>
/skill:epic-finish <epic-name>
```

Review the epic, then review each selected child proposal before implementing.
Repeat child selection, implementation, and finish until all children are
finished; only then finish the epic. `epics/<epic-name>/EPIC.md` coordinates the
outcome, with at most one active child under its `changes/` directory. There is
no need to create an epic for a standalone change.

Follow the skill files for the exact artifact formats and lifecycle gates.
Proposals name the existing documentation pages they will update, using the
[documentation ownership guide](README.md#maintaining-this-documentation).
Record new durable architectural reasoning in `docs/adr/` when needed; do not
create ADRs for routine implementation details or use change files as permanent
documentation. If a proposal conflicts with the vision, resolve the product
decision explicitly rather than weakening the vision to make it fit.

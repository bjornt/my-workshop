# AGENTS.md

Guidance for AI coding agents working in this repository.

## Project

`my-workshop` bootstraps, git-ignores, and launches a `workshop` development
environment. See `README.md` for user-facing purpose and usage. The logic lives
in unexported packages under `internal/`; `cmd/my-workshop/main.go` is a thin
entrypoint shim that delegates to `internal/cli.Run`.

## Environment & commands

The project is a standard Go module (`github.com/bjornt/my-workshop`, Go >= 1.23)
with no third-party dependencies. A `Makefile` wraps the common tasks.

|Task|Command|
|---|---|
|Build the binary|`make build` (`go build -o my-workshop ./cmd/my-workshop`)|
|Run the whole test suite|`go test ./...`|
|Run one package's tests|`go test ./internal/cli/...`|
|Format, vet, and test|`make check`|
|Run the entrypoint|`go run ./cmd/my-workshop --help`|

Keep the module dependency-free: nothing in `go.mod`'s `require` block unless
there is a strong reason. Use the standard library. `gofmt` all code (`make fmt`
or `gofmt -w .`); CI fails on unformatted files.

## Architecture

Each package owns one concern; keep them separated. Do not fold subprocess calls
or git side effects back into the pure logic.

|Package|Responsibility|
|---|---|
|`internal/yamlconfig`|Pure line-editing of the workshop YAML. No I/O beyond reading/writing the target file; no subprocess.|
|`internal/additions`|Load/parse the external additions config (`workshop.my.yaml` next to the workshop YAML, or `~/.config/my-workshop/my.yaml`). Pure parsing; file I/O only in `LoadAdditions`. Returns the empty/noop `Config` when no file is found.|
|`internal/worktree`|Hide/reveal the YAML from git. Shells out to the real `git` binary; degrades to a no-op when git is absent or the path is outside a repo.|
|`internal/workshop`|The `Workshop` interface with a `RealWorkshop` os/exec implementation; `Provision(ws, spec)`/`Hostname(ws)` orchestrate against any `Workshop`-shaped value. `ParseHostname`, `ParseWorkshopName`, and `ParseMountTarget` are pure.|
|`internal/cli`|Flag parsing (`ParseArgs`) + `Run(argv, ws, log)`.|

### Seams — preserve them

These injection points exist so the code is testable without mocks. Keep them
when you extend the code:

- Side-effecting functions (`EnsureYAML`, `HideInWorktree`, `Revert`, and the
  `RealWorkshop`) take a `Logger` (`func(string)`) parameter instead of writing
  to stdout directly — tests pass a closure that appends to a slice to capture
  output. A plain `func(string)` literal is assignable to every package's
  `Logger` type, so the CLI threads one printer through all of them.
- `cli.Run(argv []string, ws workshop.Workshop, log yamlconfig.Logger)` accepts
  an injected `Workshop` backend (pass `nil` to build the real one) and a
  logger. `Provision(ws, spec)` / `Hostname(ws)` take the workshop value as an
  argument rather than constructing the real one.

If you add a new external dependency (another CLI, network call, etc.), add a
similar seam and a fake — do not reach for `os/exec` inline in logic that tests
will need to exercise.

## Testing

Tests live next to the code they cover (`*_test.go`). Shared test helpers live
under `internal/testsupport/`. The overriding rule:

> **No mocks.** Do not stub our own functions. Exercise real behaviour through
> the seams and real external tools instead.

Instead:

- **Fakes for external tools.** Anything that talks to the `workshop` CLI is
  driven by `fakeworkshop.New(...)` in `internal/testsupport/fakeworkshop`,
  injected via `cli.Run(..., ws, ...)` / `workshop.Provision(fake, ...)`. When
  you add a new external integration, follow this pattern: build a fake that
  simulates the observable behaviour, don't mock the call.
- **Real git for worktree behaviour.** Use the helpers in
  `internal/testsupport/gitenv` (`gitenv.NewRepo(t)` builds a real temporary
  repo and chdirs into it) so tests verify actual `git status` /
  `skip-worktree` / `.git/info/exclude` effects.
- Reuse the `gitenv`/`fakeworkshop` helpers — don't re-declare them per package.
  For tests that need to run in a scratch cwd, use `gitenv.NewTmp(t)`.

Assert observable behaviour and invariants, not implementation details or
default values. Every behavioural change must ship with a test that would fail
without it. Run `go test ./...` before finishing.

Note: this targets Go 1.23, so do **not** use `t.Chdir` (Go 1.24+); the
`gitenv` helpers change directory and register a `t.Cleanup` to restore it.

## Conventions

- Match the existing style: idiomatic Go, `gofmt`-clean, package doc comments,
  standard-library only.
- Prefer editing an existing package over adding a new one. Remove obsolete code
  rather than leaving shims or dead branches.
- Don't create documentation files unless asked.

# my-workshop

Bootstrap, git-ignore, and start a [workshop](https://workshop.dev) development
environment in one command.

## What it does

`my-workshop` wraps the standard `workshop` launch flow so a project's
git-tracked `workshop.yaml` can be augmented with the SDKs, copies, and
connections *you* need — without those local additions ever showing up in
`git status`, getting committed, or being pushed. The additions come from a
separate config file (`workshop.my.yaml`), not from anything hardcoded, so each
project (or user) controls its own setup.

A normal run:

1. **Locates** the workshop YAML — an explicit path, `./workshop.yaml`, or a
   single `*.yaml`/`*.yml` file under `.workshop/`.
2. **Loads the additions config** — `workshop.my.yaml` next to the workshop
   YAML, or `~/.config/my-workshop/my.yaml` (the local file wins). When neither
   exists the tool is a noop: no SDKs are added, nothing is copied or connected.
3. **Creates or augments** the workshop YAML — writes one from a template if
   absent, otherwise merges any missing SDKs (and their plugs/slots) declared in
   the additions config into the existing file while preserving its comments,
   ordering, and formatting.
4. **Hides it from git** — a *tracked* file gets git's `skip-worktree` bit; an
   *untracked* file is added to `.git/info/exclude`. The local additions file,
   when present, is hidden the same way. Either way the change is local to your
   work tree and is never committed or pushed. Outside a git repository this is
   a silent no-op.
5. **Launches** — runs the `launch → copy → connect` sequence described by the
   additions config and prints how to connect.

## Additions config

The additions config supplies everything my-workshop used to hardcode: the base
image, the SDKs to inject (with their plugs/slots), and the
`launch → copy → connect` provision steps. Copy the sample and edit it:

```console
$ cp workshop.my.yaml.example workshop.my.yaml
```

The file is looked up in this order (first hit wins):

1. `workshop.my.yaml` next to the resolved workshop YAML.
2. `~/.config/my-workshop/my.yaml`.

When no file is found, my-workshop still creates and hides the workshop YAML but
performs no copies or connections. See `workshop.my.yaml.example` for the full
shape (`base`, `sdks`, and a `provision` block of `copy`/`connect` entries).

## Requirements

- [Go](https://go.dev/) >= 1.23 (to build; the resulting binary is a single
  static executable with no runtime dependencies)
- The [`workshop`](https://workshop.dev) CLI on your `PATH` (needed only to
  actually launch an environment; not required to run the tests)
- `git` (optional — used to hide the YAML; absent git degrades gracefully)

## Installing

Build a single self-contained binary and drop it anywhere on your `PATH`:

```console
$ go build -o my-workshop ./cmd/my-workshop
$ ./my-workshop --help
```

Or install straight into your Go bin directory:

```console
$ go install github.com/bjornt/my-workshop@latest
```

## Usage

```console
$ my-workshop                    # auto-detect the YAML and launch
$ my-workshop path/to/dev.yaml   # use an explicit YAML path
$ my-workshop --base ubuntu@24.04  # base image for a newly created YAML
$ my-workshop --revert           # stop ignoring the YAML and exit (no launch)
```

### Options

| Flag / arg      | Description                                                                 |
| --------------- | --------------------------------------------------------------------------- |
| `PATH`          | Path to the workshop YAML (default: auto-detect).                           |
| `--base IMAGE`  | Base image for a newly created `workshop.yaml` (default: the additions config's `base`, or `ubuntu@24.04`). |
| `--revert`      | Stop ignoring the YAML and exit without launching: clears `skip-worktree` for a tracked file (restoring the committed version), or drops the `.git/info/exclude` entry for an untracked one. |

## Development

Build, format, vet, and test with the `Makefile` targets:

```console
$ make build     # go build -o my-workshop ./cmd/my-workshop
$ make test      # go test ./...
$ make check     # gofmt -w . && go vet ./... && go test ./...
```

The logic lives in unexported packages under `internal/`; `cmd/my-workshop/main.go` is a thin
entrypoint shim that delegates to `internal/cli.Run`.

| Package                    | Responsibility                                              |
| -------------------------- | ----------------------------------------------------------- |
| `internal/additions`       | Load the external additions config (`workshop.my.yaml`).    |
| `internal/yamlconfig`      | Locate, create, and augment the workshop YAML.              |
| `internal/worktree`        | Hide/reveal the YAML from git (shells out to real `git`).   |
| `internal/workshop`        | `Workshop` CLI wrapper + `Provision`/`Hostname` flow.       |
| `internal/cli`             | Argument parsing and `Run()`.                               |

## Running the tests

```console
$ go test ./...
```

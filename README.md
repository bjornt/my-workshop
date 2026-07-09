# my-workshop

Bootstrap, git-ignore, and start a [workshop](https://workshop.dev) development
environment in one command.

## What it does

`my-workshop` wraps the standard `workshop` launch flow so a project's
git-tracked `workshop.yaml` can be augmented with the SDKs *you* need — without
those local additions ever showing up in `git status`, getting committed, or
being pushed.

A normal run:

1. **Locates** the workshop YAML — an explicit path, `./workshop.yaml`, or a
   single `*.yaml`/`*.yml` file under `.workshop/`.
2. **Creates or augments** it — writes one from a template if absent, otherwise
   merges any missing required SDKs (and their plugs/slots) into the existing
   file while preserving its comments, ordering, and formatting.
3. **Hides it from git** — a *tracked* file gets git's `skip-worktree` bit; an
   *untracked* file is added to `.git/info/exclude`. Either way the change is
   local to your work tree and is never committed or pushed. Outside a git
   repository this is a silent no-op.
4. **Launches** — runs the `launch → stop → remount → connect → start` sequence
   and prints how to connect.

The required SDKs are declared in `my_workshop/yaml_config.py` (`REQUIRED_SDKS`):
`try-zed-remote`, `try-omp` (with a `pi-auth-gateway` tunnel plug), and the
`system` slot that gateway connects to.

## Requirements

- Python >= 3.9
- The [`workshop`](https://workshop.dev) CLI on your `PATH` (needed only to
  actually launch an environment; not required to run the tests)
- `git` (optional — used to hide the YAML; absent git degrades gracefully)
- [`uv`](https://docs.astral.sh/uv/) for development

## Usage

Run the entrypoint directly:

```console
$ ./my-workshop                 # auto-detect the YAML and launch
$ ./my-workshop path/to/dev.yaml  # use an explicit YAML path
$ ./my-workshop --base ubuntu@24.04  # base image for a newly created YAML
$ ./my-workshop --revert        # stop ignoring the YAML and exit (no launch)
```

Or, in a `uv`-managed checkout, via the installed console script:

```console
$ uv run my-workshop --help
```

### Options

| Flag / arg      | Description                                                                 |
| --------------- | --------------------------------------------------------------------------- |
| `PATH`          | Path to the workshop YAML (default: auto-detect).                           |
| `--base IMAGE`  | Base image for a newly created `workshop.yaml` (default: `ubuntu@24.04`).    |
| `--revert`      | Stop ignoring the YAML and exit without launching: clears `skip-worktree` for a tracked file (restoring the committed version), or drops the `.git/info/exclude` entry for an untracked one. |

## Development

The project is set up with [`uv`](https://docs.astral.sh/uv/). Create the
environment (installs the package plus the test dependencies) with:

```console
$ uv sync
```

The logic lives in the importable `my_workshop` package:

| Module                      | Responsibility                                              |
| --------------------------- | ----------------------------------------------------------- |
| `my_workshop/yaml_config.py`| Locate, create, and augment the workshop YAML.              |
| `my_workshop/worktree.py`   | Hide/reveal the YAML from git (shells out to real `git`).   |
| `my_workshop/workshop.py`   | `Workshop` CLI wrapper + `provision`/`hostname` flow.       |
| `my_workshop/cli.py`        | Argument parsing and `main()`.                              |

## Running the tests

```console
$ uv run pytest
```

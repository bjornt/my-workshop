# my-workshop

Add your own tools to a [Workshop](https://ubuntu.com/workshop) development
environment, keep the configuration local, and launch it in one command.

`my-workshop` complements a project's Workshop setup with the SDKs, configuration
copies, and connections you choose. It can also create a Workshop YAML for a
project that does not have one. The primary use case is running your preferred
AI coding harness inside Workshop.

## What it does

A normal run:

1. Finds the Workshop YAML: an explicit path, `./workshop.yaml`, or a single
   YAML file under `.workshop/`. If none exists, it creates `workshop.yaml`.
2. Loads `workshop.my.yaml` beside that YAML, falling back to
   `~/.config/my-workshop/my.yaml`. The files are not combined.
3. Adds missing SDKs, plugs, and slots without replacing existing named entries.
   Editing is line-oriented, not a general YAML merge.
4. Attempts to hide the Workshop YAML and the selected additions file from Git:
   `skip-worktree` for tracked files, local excludes for untracked files.
5. Runs `workshop launch`, copies configured directories into SDK mounts,
   connects configured plugs and slots, and prints connection instructions.

Without an additions file, it still creates or keeps the Workshop YAML,
attempts to hide it, and launches Workshop; it just adds no SDKs and performs
no copies or connections.

## Requirements

- [Workshop](https://ubuntu.com/workshop), installed and configured with its
  prerequisites, on `PATH`. See the
  [upstream setup guide](https://ubuntu.com/workshop/docs/tutorial/part-1-get-started/).
- [Go](https://go.dev/) 1.23 or later to build or install `my-workshop`.
  The binary does not need Go at runtime.
- `git` for local Git hiding. Without it, launch can still proceed, but files
  are not hidden.
- `tar` on the host and inside the Workshop environment if you configure
  directory copies.

## Install

Install into your Go binary directory, and ensure that directory is on `PATH`:

```sh
go install github.com/bjornt/my-workshop/cmd/my-workshop@latest
```

Or build from this checkout:

```sh
go build -o my-workshop ./cmd/my-workshop
./my-workshop --help
```

Place the resulting binary on `PATH` to use it from other projects.

## Quick start

From the project directory where you would normally run `workshop launch`:

```sh
my-workshop
workshop shell
```

To add personal tools, create `workshop.my.yaml` beside the resolved Workshop
YAML, or use `~/.config/my-workshop/my.yaml` for your cross-project configuration.
Start with the [configuration reference](docs/configuration.md) and adapt the
[example additions file](workshop.my.yaml.example). Check SDK availability,
source directories, and host-service access before using its copy/connect steps.

```sh
my-workshop --base ubuntu@24.04  # base for a newly created YAML only
my-workshop --help
```

See [usage and recovery](docs/usage.md) for path selection, CLI options, launch
behavior, and troubleshooting. An explicit YAML path selects the file this
wrapper edits; it is not forwarded to Workshop and does not change the working
directory.

## Important limits

- Git hiding is best effort, not a guarantee against commits. It does not
  unstage changes or provide a backup.
- **`my-workshop --revert` can discard local edits.** For a tracked Workshop YAML
  or selected additions file, it clears `skip-worktree` and checks out the file
  from Git's index. Untracked files are kept and their local exclude entries
  removed. It does not undo copies or connections. Read
  [revert behavior](docs/usage.md#reverting-local-hiding) before using it.
- Configured copies and connections run on every invocation, not just initial
  setup. Copies can overwrite destination files.
- The selected additions file is trusted: its copy and connection steps execute
  without a separate approval prompt. Review project-local additions before
  running the tool. Isolation depends on Workshop and the resources you expose.

## Documentation and development

- [Documentation index](docs/README.md): current user and contributor guides.
- [Vision](docs/VISION.md): product direction, principles, and boundaries;
  not a promise that every desired capability is implemented.
- [Development guide](docs/development.md): architecture, testing, and the
  feature-definition workflow in `.agents/skills/`.

```sh
make build  # build ./my-workshop
make check  # format, vet, and test
```

# Usage and runtime behavior

[Documentation index](README.md) · [Additions configuration](configuration.md)

`my-workshop` prepares a workshop YAML, attempts to hide that YAML and the
selected additions file from Git, then invokes the external `workshop` CLI.
It modifies files in place; it does not create an isolated copy of the project
or keep a backup of the original files.

## Before running

- Install and configure the external `workshop` CLI so that `workshop launch`
  works from the intended project directory. The binary must be on `PATH`.
- For configured copies, install `tar` on the host and make `tar` available
  inside the workshop through `workshop exec`.
- Hostname fallback uses `hostname -I` inside the workshop. It is unnecessary
  when `workshop info` supplies a nonempty top-level hostname.
- Git is optional, but hiding and restoring files require a working `git`
  executable and the correct current repository.
- Save an independent copy of any YAML or additions-file changes you cannot
  afford to lose. In particular, read the warning under [`--revert`](#reverting-local-hiding)
  before using it.

The wrapper does not install these dependencies or configure the Workshop
backend. `--help` and `--revert` do not launch Workshop. Neither `ssh` nor
`workshop shell` is run automatically; they appear only in the final hint.

## Command line

```text
my-workshop [--base IMAGE] [--revert] [PATH]
```

| Argument | Current behavior |
| --- | --- |
| `PATH` | Selects the YAML that this wrapper creates, augments, hides, or reverts. Omit it for discovery described below. |
| `--base IMAGE` or `--base=IMAGE` | Sets the base used when creating a missing YAML. It does not replace an existing file's base. See [base precedence](configuration.md). |
| `--revert` | Performs Git restoration/unhiding for the selected YAML and currently selected additions file, then exits without launching. **Tracked-file restoration can discard your edits.** |
| `-h` or `--help` | Prints help to standard output and exits successfully, before looking up files. |

Flags can precede or follow `PATH`, for example:

```sh
my-workshop
my-workshop --base ubuntu@24.04
my-workshop .workshop/dev.yaml
my-workshop .workshop/dev.yaml --revert
```

The last example is destructive for tracked files; it is not a preview.
The parser also accepts the single-dash forms `-base` and `-revert`. Only the
first positional argument is used; extra positional arguments are not rejected
and are not forwarded to Workshop. Unknown options and invalid option values
are errors. This is not a general-purpose pass-through for `workshop` options.
For a filename beginning with `-`, use a relative spelling such as
`./-dev.yaml`: argument reordering does not preserve conventional `--`
end-of-options handling in all cases.

Reported command-line, YAML read/write, launch, copy, and connect errors are
printed to standard error and exit with status 1. Success and help exit with
status 0. Git failures and hostname lookup failures are not reliably reflected
in that exit status; see below.

## Selecting the YAML

Discovery is relative to the process's **current working directory**, not a
search upwards through parent directories:

1. Use `PATH` verbatim when supplied, whether or not it exists yet.
2. Otherwise prefer `./workshop.yaml` when its filesystem lookup succeeds.
3. Otherwise collect `.workshop/*.yaml` and `.workshop/*.yml`, without recursion.
   Use the sole match.
4. If there are no matches, select `./workshop.yaml`, which a normal run will
   create.

More than one `.workshop/` match is an error listing the sorted candidates and
asking for an explicit path. This also applies to `--revert`; nothing is
reverted until selection succeeds. A root `workshop.yaml` takes precedence over
any number of `.workshop/` matches. Root `workshop.yml` is not auto-detected.

There is no special exclusion for additions files. For example,
`.workshop/dev.yaml` alongside `.workshop/workshop.my.yaml` makes automatic
selection ambiguous; an additions file alone can itself be selected as the
workshop YAML. Use `my-workshop .workshop/dev.yaml` in that layout, subject to
the runtime-path limitation below.

The wrapper does not validate that a discovered candidate is a regular file or
that an explicit path exists during selection. A normal run will try to create
a missing file, but will not create missing parent directories. Directories,
unreadable files, and unwritable locations can therefore fail later during
YAML preparation. See [configuration](configuration.md) for creation and merge
semantics, including the supported YAML layout.

### Important: `PATH` does not select the external Workshop instance

The selected path controls **this wrapper's file operations and additions-file
lookup only**. It is not forwarded to `workshop launch`, `workshop info`,
`workshop exec`, or `workshop connect`, and the wrapper does not change its
working directory.

All those commands run from the directory where you invoked `my-workshop`.
The implementation therefore assumes that the external Workshop CLI's own
selection from that directory refers to the same intended environment. An
explicit path such as `other-project/dev.yaml` is not a reliable way to switch
the launched environment: it can edit that file while Workshop launches
something else. Run from the appropriate project directory and establish what
Workshop itself will select; do not infer that selection from this wrapper's
`PATH` argument.

## What a normal run does

A run proceeds in this order:

1. Select the YAML and locate/load the additions configuration.
2. Create or augment the YAML using that configuration.
3. Attempt Git hiding for the YAML and, if found, the selected additions file.
4. Run `workshop launch` and wait for it to finish.
5. Read `workshop info` once for the workshop name, mount destinations, and
   hostname.
6. Perform every configured copy, sequentially in configuration order.
7. Perform every configured connection, sequentially in configuration order.
8. Choose the hostname from the saved info output, or fall back to
   `workshop exec hostname -I`, then print a connection hint.

All copies happen before any connections. See [configuration](configuration.md)
for additions lookup, schema, and YAML merge behavior.

### No additions file is still a launch

The message `No additions config found; running as noop.` refers to optional
additions, not the whole command. Without additions, the wrapper still creates
a missing YAML, attempts Git hiding, launches Workshop, reads its info, and
prints a connection hint. It adds no SDKs and performs no copies or connections;
an existing YAML is left unchanged. There is no implicit built-in provisioning
recipe. An unreadable selected additions file also loads as empty configuration;
see [configuration loading limitations](configuration.md).

### Copies resolve an SDK mount, not an arbitrary destination path

A copy target such as `omp:omp-home` means SDK `omp`, mount `omp-home`. The
wrapper reads that mount's `workshop-target` value from `workshop info`; it does
not treat the target string as a destination directory. Mount lookup is a
line-oriented parser expecting the SDK at two spaces of indentation, the mount
name at six, and `workshop-target` at eight. A different output layout can
prevent resolution.

For each entry, the real backend effectively runs this pipeline, with arguments
passed directly to subprocesses rather than through a shell:

```text
tar -cf - -C SOURCE . | workshop exec -- tar -xf - -C DESTINATION
```

This copies the source directory's contents, including hidden entries, into the
existing destination. It can overwrite destination files. There is no filtering
of credentials or other sensitive files, destination-directory creation, backup,
or rollback.

Source handling is deliberately limited:

- `~` and a leading `~/` are expanded using `HOME`, with the OS home directory
  lookup as fallback.
- Relative sources are relative to the invocation directory, **not** to the
  additions file or selected YAML, even when those files are elsewhere.
- `~otheruser`, environment-variable references such as `$HOME`, and wildcard
  patterns are not expanded by the wrapper.

Sources and destinations are not checked before launch. A missing source is not
silently skipped: the host `tar` command attempts it and its failure is returned.
A missing SDK, missing mount, or failed info command leaves the destination
empty; the wrapper still invokes the copy with an empty `-C` argument rather
than reporting a dedicated missing-mount error. Any copy error stops subsequent
copies and prevents all connections. Files already extracted, earlier copies,
the launched workshop, and local YAML/Git changes remain in place.

### Connections use the name from `workshop info`

Given a top-level `name: dev`, the following configured pair:

```text
plug: omp:pi-auth-gateway
slot: system:pi-auth-gateway
```

becomes:

```sh
workshop connect dev/omp:pi-auth-gateway dev/system:pi-auth-gateway
```

The wrapper prefixes both endpoints; configure them without the workshop-name
prefix. It does not verify the SDKs, plugs, or slots first. If info fails or has
no nonempty top-level name, the prefix is empty and the attempted endpoints
start with `/`. A connect error stops later connections without undoing earlier
connections or copies.

### Hostname and output limitations

The first nonempty, unindented `hostname:` value in the saved info output is
preferred. If none is available, the wrapper runs `workshop exec hostname -I`
and uses the first whitespace-separated field. It does not refresh info after
provisioning or test DNS, SSH access, or network reachability.

An unsuccessful or empty fallback yields an empty hostname without failing the
run. The final message can consequently suggest the incomplete command
`ssh workshop@`. Inspect `workshop info` and the environment yourself; a printed
hint is not proof that a connection will succeed.

Launch, connect, and copy command traces are logged, but the real backend does
not attach the child processes' standard output/error to the terminal. Info and
hostname output are captured internally. A failed subprocess may therefore
produce only a generic error such as `exit status 1`. Run the relevant external
Workshop or tar command directly from the same directory to obtain diagnostics.
The wrapper does not open an interactive session.

## Git hiding is best effort, not protection

For each selected file, the wrapper uses Git in the **invocation directory**:

- If tracked, it attempts `git update-index --skip-worktree -- PATH`. This is
  intended to suppress ordinary reporting/staging of local working-tree edits.
- If untracked and inside the current repository, it adds a repository-root
  anchored pattern, such as `/workshop.yaml`, to Git's local `info/exclude`
  file. It avoids adding an identical line twice and does not modify the
  project's `.gitignore`.
- It applies the same logic to the selected additions file, including a local
  `workshop.my.yaml`. A global additions file is also attempted, but is usually
  outside the current repository and receives no hiding there.
- With no usable Git or outside a repository, hiding does nothing. A selected
  file outside the current repository is not handled by switching to that
  file's repository, and cannot get a local exclude entry in the current one.

This changes Git metadata, not file access permissions. It neither encrypts nor
isolates the files, and is not a guarantee that local additions cannot be
committed, pushed, or otherwise disclosed. Pre-existing staged changes are not
removed from the index. Tracked-file Git command failures are ignored, and the
wrapper can print an “ignoring” message even if setting the bit failed.
Exclusion failures also do not stop launch.

Inspect actual Git state rather than treating the log as confirmation. For
example, `git ls-files -v -- workshop.yaml` normally shows an `S` marker when
skip-worktree is set; `git check-ignore -v -- workshop.my.yaml` can identify an
untracked file's matching ignore rule. A clean `git status` alone does not show
whether a tracked file contains hidden local edits.

## Reverting local hiding

**Back up the selected YAML and additions file before `--revert` if either
contains work you want to keep. This is not an undo of only my-workshop's
changes.**

```sh
my-workshop --revert workshop.yaml
```

After selecting the YAML and discovering the additions file, this command:

- For each tracked file, clears skip-worktree when detected, then executes
  `git checkout -- PATH`. That restores the working-tree file from the index
  (normally the last committed content, unless a different version is staged).
  It discards all unstaged changes to that file, including edits made by you
  before or after running the wrapper. Restoration is attempted even if the
  file was never hidden by my-workshop. It does not reset staged content.
- For each untracked file, removes the exact local exclude line for its path.
  **The file and its contents are retained.** Other ignore rules may still
  hide it.
- Handles the additions file selected **now**, not a recorded list from a
  previous run. If files were moved or additions precedence changed, previously
  hidden files may need manual attention.
- Exits without creating/merging YAML, launching Workshop, copying, connecting,
  or printing the connection hint. Outside a repository, it reports nothing to
  revert.

As with hiding, Git command failures are not propagated. A restoration message
and status 0 are not proof that the file was restored; inspect the file and Git
state afterward.

To expose a tracked file for review **without discarding its current contents**,
do not use `--revert`. After making a backup, clear only its bit yourself, then
review the diff:

```sh
git update-index --no-skip-worktree -- workshop.yaml
git diff -- workshop.yaml
```

Repeat for a tracked additions file if necessary. For an untracked file,
removing its matching line from the local Git exclude file preserves it, as
does `--revert`; remember that the latter also processes the selected YAML and
may restore that file destructively. A later normal run will attempt hiding
again.

## Recovering from a failed run

There is no transaction or automatic cleanup. YAML preparation happens before
Git hiding and launch; launch happens before copies and connections. A later
failure leaves earlier operations in place, and a copy itself can leave partial
destination contents. Failed info and hostname lookups are tolerated as
described above rather than producing a dedicated failure.

1. Save local files and inspect which launch, copy, or connect step failed.
2. Check the actual Workshop environment from the original invocation directory,
   including `workshop info`, mount destinations, and any copied contents.
3. Fix the source path, configuration, dependency, or external Workshop problem
   before retrying. Re-running starts the launch/provision sequence again; it
   does not resume at the failed step or skip copies/connections already done.
4. If you only want to reveal local Git changes, use the preservation guidance
   above. `--revert` does not stop/delete the workshop, remove copied files, or
   disconnect endpoints.

## Implementation references

These behaviors come from [CLI orchestration](../internal/cli/cli.go),
[YAML discovery](../internal/yamlconfig/yamlconfig.go),
[Workshop execution](../internal/workshop/workshop.go), and
[Git integration](../internal/worktree/worktree.go). Existing tests cover
[CLI flows](../internal/cli/cli_test.go),
[discovery](../internal/yamlconfig/yamlconfig_test.go),
[runtime parsing and order](../internal/workshop/workshop_test.go), and
[tracked/untracked Git behavior](../internal/worktree/worktree_test.go).

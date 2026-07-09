"""Bootstrap, ignore, and start a workshop environment.

Wraps the standard 'workshop' launch flow so a project's git-tracked
workshop YAML can be augmented with the SDKs this developer needs without
ever appearing as a local modification in git.

A normal run:

  1. Locates the workshop YAML (an explicit path, ./workshop.yaml, or a
     single file under .workshop/).
  2. Loads external additions config (workshop.my.yaml or
     ~/.config/my-workshop/my.yaml).  When no file is found the tool is a
     noop: no SDKs added, no files copied, no connections made.
  3. Creates the workshop YAML from a template if absent; otherwise merges
     any missing SDKs and their plugs/slots into the existing file.
  4. Hides the file from git so it never appears in 'git status': a tracked
     file gets the skip-worktree bit; an untracked file is added to
     .git/info/exclude. Either way the change is local to your work tree and
     is never committed or pushed.
 5. Runs the launch/copy/connect sequence and prints how to connect.

Use --revert to stop ignoring the YAML: it clears skip-worktree and restores a
tracked file, or drops the local exclude entry for an untracked one.
"""

import argparse
import os
import sys

from .additions import find_additions, load_additions
from .workshop import Workshop, provision
from .worktree import hide_in_worktree, revert
from .yaml_config import ensure_yaml, find_yaml


def build_parser(prog=None):
    ap = argparse.ArgumentParser(
        prog=prog,
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    ap.add_argument(
        "--base",
        default=None,
        metavar="IMAGE",
        help="Base image for a new workshop.yaml "
        "(default: additions config, or ubuntu@24.04)",
    )
    ap.add_argument(
        "--revert",
        action="store_true",
        help="Stop ignoring the workshop YAML and exit without launching: "
        "clears skip-worktree and restores a tracked file, or drops the local "
        ".git/info/exclude entry for an untracked one.",
    )
    ap.add_argument(
        "yaml",
        nargs="?",
        metavar="PATH",
        help="Path to the workshop YAML file "
        "(default: auto-detect workshop.yaml or a single file under .workshop/)",
    )
    return ap


def main(argv=None, workshop=None):
    prog = os.path.basename(sys.argv[0]) if argv is None else None
    args = build_parser(prog).parse_args(argv)
    prog = prog or "my-workshop"

    path = find_yaml(args.yaml)
    additions_path = find_additions(path)

    if args.revert:
        revert(path, prog)
        if additions_path:
            revert(additions_path, prog)
        return

    if additions_path:
        print(f"Using additions config: {additions_path}")
    else:
        print("No additions config found; running as noop.")
    additions = load_additions(path)
    base = args.base or additions.get("base") or "ubuntu@24.04"
    ensure_yaml(path, base, additions.get("sdks", []))
    hide_in_worktree(path, prog)
    if additions_path:
        hide_in_worktree(additions_path, prog)

    ws = workshop if workshop is not None else Workshop()
    host = provision(ws, additions.get("provision", {}))
    print(f"\nTo connect, use 'workshop shell' or 'ssh workshop@{host}'")


if __name__ == "__main__":
    main()

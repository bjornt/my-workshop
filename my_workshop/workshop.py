"""Drive the `workshop` CLI launch flow.

`Workshop` is a thin wrapper over the real `workshop` command-line tool. The
orchestration helpers (`provision`, `hostname`) take any object with the same
interface, so tests can drive them with a fake that never spawns a process.
"""

import os
import subprocess


def parse_hostname(info_output):
    """Extract the top-level 'hostname:' value from `workshop info` output.

    'workshop info' prints a top-level 'hostname:' line only when the backend
    has assigned one; older bases without DNS omit it. Indented SDK-detail
    lines are skipped. Returns None when no such line is present.
    """
    for line in info_output.splitlines():
        if line[:1].isspace():   # skip indented SDK detail lines
            continue
        key, sep, value = line.partition(":")
        if sep and key.strip() == "hostname" and value.strip():
            return value.strip()
    return None


def parse_workshop_name(info_output):
    """Extract the top-level 'name:' value from `workshop info` output.

    Returns None when no top-level ``name:`` line is present.
    """
    for line in info_output.splitlines():
        if line[:1].isspace():   # skip indented SDK detail lines
            continue
        key, sep, value = line.partition(":")
        if sep and key.strip() == "name" and value.strip():
            return value.strip()
    return None


def parse_mount_target(info_output, sdk, mount):
    """Extract the workshop-target path for a named SDK mount.

    'workshop info' prints SDKs at indent 2, mounts at indent 6, and
    mount details (including 'workshop-target:') at indent 8.  Returns
    None when the SDK or mount is not found.
    """
    current_sdk = None
    current_mount = None
    for line in info_output.splitlines():
        indent = len(line) - len(line.lstrip())
        stripped = line.strip()
        if indent == 2 and not stripped.startswith(("mounts:", "tracking:", "installed:")):
            current_sdk = stripped.rstrip(":")
            current_mount = None
        elif indent == 6 and current_sdk == sdk:
            current_mount = stripped.rstrip(":")
        elif indent == 8 and current_mount == mount:
            key, sep, value = stripped.partition(":")
            if sep and key == "workshop-target":
                return value.strip()
    return None


class Workshop:
    """Thin wrapper over the real `workshop` command-line tool."""

    def __init__(self, log=print):
        self._log = log

    def _run(self, *args):
        self._log(f"+ workshop {' '.join(args)}")
        subprocess.run(["workshop", *args], check=True)

    def launch(self):
        """Run ``workshop launch`` and wait for it to finish."""
        self._run("launch")

    def copy_to(self, source, dest):
        """Copy a host directory into the workshop via a tar pipe."""
        self._log(f"+ tar -cf - -C {source} . | workshop exec -- tar -xf - -C {dest}")
        tar_send = subprocess.Popen(
            ["tar", "-cf", "-", "-C", source, "."],
            stdout=subprocess.PIPE,
        )
        recv = subprocess.Popen(
            ["workshop", "exec", "--", "tar", "-xf", "-", "-C", dest],
            stdin=tar_send.stdout,
        )
        tar_send.stdout.close()
        recv.wait()
        tar_send.wait()
        if tar_send.returncode != 0:
            raise subprocess.CalledProcessError(tar_send.returncode, "tar")
        if recv.returncode != 0:
            raise subprocess.CalledProcessError(recv.returncode, "workshop exec")

    def connect(self, plug, slot):
        """Connect a plug to a slot."""
        self._run("connect", plug, slot)

    def info(self):
        """Return the output of ``workshop info``, or None on failure."""
        result = subprocess.run(
            ["workshop", "info"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            return None
        return result.stdout

    def exec(self, *cmd):
        """Run a command inside the workshop; return its stdout."""
        result = subprocess.run(
            ["workshop", "exec", *cmd],
            capture_output=True,
            text=True,
            check=True,
        )
        return result.stdout


def hostname(ws, info=None):
    """Best-effort hostname for a launched workshop.

    Prefer the DNS name reported by `workshop info`; fall back to the first IP
    address from `hostname -I` run inside the workshop.  Pass *info* to reuse
    an earlier `ws.info()` result and avoid a redundant query.
    """
    if info is None:
        info = ws.info()
    if info is not None:
        host = parse_hostname(info)
        if host is not None:
            return host
    return ws.exec("hostname", "-I").split()[0]


def provision(ws, provision_spec):
    """Run the launch/copy/connect sequence.

    *provision_spec* is the ``provision`` sub-dict from the additions config,
    containing ``copy`` and ``connect`` lists.

    Returns the workshop's hostname once it is up.
    """
    ws.launch()
    info = ws.info()
    name = parse_workshop_name(info)

    for entry in provision_spec.get("copy", []):
        source = os.path.expanduser(entry["source"])
        sdk, _, mount = entry["target"].partition(":")
        dest = parse_mount_target(info, sdk, mount)
        ws.copy_to(source, dest)

    for entry in provision_spec.get("connect", []):
        plug = f"{name}/{entry['plug']}"
        slot = f"{name}/{entry['slot']}"
        ws.connect(plug, slot)

    return hostname(ws, info)

"""Drive the `workshop` CLI launch flow.

`Workshop` is a thin wrapper over the real `workshop` command-line tool. The
orchestration helpers (`provision`, `hostname`) take any object with the same
interface, so tests can drive them with a fake that never spawns a process.
"""

import subprocess

# The remount source/dest and the plug/slot wiring for the dev workshop.
OMP_HOME_MOUNT = "dev/omp:omp-home"
OMP_GATEWAY_PLUG = "dev/omp:pi-auth-gateway"
SYSTEM_GATEWAY_SLOT = "dev/system:pi-auth-gateway"


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


class Workshop:
    """Thin wrapper over the real `workshop` command-line tool."""

    def __init__(self, log=print):
        self._log = log

    def _run(self, *args):
        self._log(f"+ workshop {' '.join(args)}")
        subprocess.run(["workshop", *args], check=True)

    def launch(self):
        self._run("launch")

    def stop(self):
        self._run("stop")

    def remount(self, source, dest):
        self._run("remount", source, dest)

    def connect(self, plug, slot):
        self._run("connect", plug, slot)

    def start(self):
        self._run("start")

    def info(self):
        """Return the stdout of `workshop info`, or None if the command failed."""
        result = subprocess.run(
            ["workshop", "info"], capture_output=True, text=True,
        )
        if result.returncode != 0:
            return None
        return result.stdout

    def exec(self, *cmd):
        """Run a command inside the workshop and return its stdout."""
        result = subprocess.run(
            ["workshop", "exec", "--", *cmd],
            check=True, capture_output=True, text=True,
        )
        return result.stdout


def hostname(ws):
    """Best-effort hostname for a launched workshop.

    Prefer the DNS name reported by `workshop info`; fall back to the first IP
    address from `hostname -I` run inside the workshop.
    """
    info = ws.info()
    if info is not None:
        host = parse_hostname(info)
        if host is not None:
            return host
    return ws.exec("hostname", "-I").split()[0]


def provision(ws, omp_home):
    """Run the launch/stop/remount/connect/start sequence.

    Returns the workshop's hostname once it is up.
    """
    ws.launch()
    ws.stop()
    ws.remount(OMP_HOME_MOUNT, omp_home)
    ws.connect(OMP_GATEWAY_PLUG, SYSTEM_GATEWAY_SLOT)
    ws.start()
    return hostname(ws)

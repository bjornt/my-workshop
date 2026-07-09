"""A fake `Workshop` that simulates the real `workshop` CLI in-memory.

Drop-in for `my_workshop.workshop.Workshop`: same methods, no subprocess. It
models the observable behaviour the launch flow depends on -- lifecycle state,
remounts, connections, and the `info`/`exec` queries used to discover a
hostname -- so tests can drive `provision`/`hostname`/`main` without a backend.
"""


class FakeWorkshop:
    def __init__(self, hostname=None, ip="10.0.0.5", info_ok=True):
        """Configure the simulated backend.

        hostname: the DNS name `workshop info` reports (None => no hostname
                  line, forcing the `hostname -I` fallback).
        ip:       first address `hostname -I` returns inside the workshop.
        info_ok:  whether `workshop info` succeeds (False simulates a
                  non-zero exit / older base with no info).
        """
        self.calls = []          # ordered ("op", *args) log of every call
        self.remounts = []       # (source, dest) pairs passed to remount
        self.connections = []    # (plug, slot) pairs passed to connect
        self.launched = False
        self.stopped = False
        self.started = False
        self._hostname = hostname
        self._ip = ip
        self._info_ok = info_ok

    def launch(self):
        self.calls.append(("launch",))
        self.launched = True

    def stop(self):
        self.calls.append(("stop",))
        self.stopped = True
        self.started = False

    def remount(self, source, dest):
        self.calls.append(("remount", source, dest))
        self.remounts.append((source, dest))

    def connect(self, plug, slot):
        self.calls.append(("connect", plug, slot))
        self.connections.append((plug, slot))

    def start(self):
        self.calls.append(("start",))
        self.started = True

    def info(self):
        self.calls.append(("info",))
        if not self._info_ok:
            return None
        lines = ["name: dev", "base: ubuntu@24.04"]
        if self._hostname:
            lines.append(f"hostname: {self._hostname}")
        lines.append("sdks:")
        lines.append("  - name: try-omp")
        lines.append("    hostname: indented-detail-should-be-ignored")
        return "\n".join(lines) + "\n"

    def exec(self, *cmd):
        self.calls.append(("exec", *cmd))
        if cmd == ("hostname", "-I"):
            return f"{self._ip} 192.168.0.1 \n"
        return ""

    @property
    def ops(self):
        """The names of the operations invoked, in order."""
        return [c[0] for c in self.calls]

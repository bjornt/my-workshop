"""A fake `Workshop` that simulates the real `workshop` CLI in-memory.

Drop-in for `my_workshop.workshop.Workshop`: same methods, no subprocess. It
models the observable behaviour the launch flow depends on -- lifecycle state,
copies, connections, and the `info`/`exec` queries used to discover a
hostname -- so tests can drive `provision`/`hostname`/`main` without a backend.
"""


class FakeWorkshop:
    def __init__(self, hostname=None, ip="10.0.0.5", info_ok=True, name="dev"):
        """Configure the simulated backend.

        hostname: the DNS name `workshop info` reports (None => no hostname
                  line, forcing the `hostname -I` fallback).
        ip:       first address `hostname -I` returns inside the workshop.
        info_ok:  whether `workshop info` succeeds (False simulates a
                  non-zero exit / older base with no info).
        name:     the workshop name `workshop info` reports.
        """
        self.calls = []          # ordered ("op", *args) log of every call
        self.copies = []         # (source, dest) pairs passed to copy_to
        self.connections = []    # (plug, slot) pairs passed to connect
        self.launched = False
        self._hostname = hostname
        self._ip = ip
        self._info_ok = info_ok
        self._name = name

    def launch(self):
        self.calls.append(("launch",))
        self.launched = True

    def copy_to(self, source, dest):
        self.calls.append(("copy_to", source, dest))
        self.copies.append((source, dest))

    def connect(self, plug, slot):
        self.calls.append(("connect", plug, slot))
        self.connections.append((plug, slot))

    def info(self):
        self.calls.append(("info",))
        if not self._info_ok:
            return None
        lines = [f"name: {self._name}", "base: ubuntu@24.04"]
        if self._hostname:
            lines.append(f"hostname: {self._hostname}")
        lines.append("sdks:")
        lines.append("  omp:")
        lines.append("    mounts:")
        lines.append("      omp-home:")
        lines.append("        workshop-target: /home/workshop/.omp")
        lines.append("  zed-remote:")
        lines.append("    mounts:")
        lines.append("      zed-server:")
        lines.append("        workshop-target: /home/workshop/.zed_server")
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

#!/usr/bin/env python3
"""Verify smoke-runner cleanup using bounded, inert subprocess fixtures."""

import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import smoke_test


@unittest.skipUnless(os.name == "posix", "requires POSIX process groups")
class TestSmokeTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="smoke-unit-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.plugin = self.root / "cpa-live-voice.dylib"
        self.plugin.write_bytes(b"inert fixture")
        self.processes = []
        self.addCleanup(self.clean_processes)

    def clean_processes(self):
        for proc in self.processes:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            proc.wait(timeout=3)

    def fixture(self, mode):
        script = self.root / f"fake_server_{mode}.py"
        marker = self.root / "server.pid"
        code = f'''#!{sys.executable}
import os, signal, sys, time
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
Path({str(marker)!r}).write_text(str(os.getpid()))
mode = {mode!r}
if mode == "fail":
    sys.exit(2)
config = Path(sys.argv[sys.argv.index("--config") + 1]).read_text()
port = int(next(line.split(":", 1)[1] for line in config.splitlines() if line.startswith("port:")))
signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
if mode == "never_ready":
    while True: time.sleep(0.05)
if mode == "ignore":
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
if mode == "grandchild":
    ready_r, ready_w = os.pipe()
    child = os.fork()
    if child == 0:
        os.close(ready_r)
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        Path({str(self.root / "grandchild.pid")!r}).write_text(str(os.getpid()))
        os.write(ready_w, b"ready")
        os.close(ready_w)
        while True: time.sleep(0.05)
    os.close(ready_w)
    os.read(ready_r, 5)
    os.close(ready_r)
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{{"data": []}}')
    def log_message(self, *_): pass
print("plugin registered plugin_id=cpa-live-voice", flush=True)
HTTPServer(("127.0.0.1", port), Handler).serve_forever()
'''
        script.write_text(code)
        script.chmod(0o755)
        return script, marker

    def assert_gone(self, proc):
        self.assertIsNotNone(proc.poll(), "direct child must be reaped")
        self.assertFalse(smoke_test.is_group_alive(proc.pid), "process group must be empty")
        marker = self.root / "grandchild.pid"
        if marker.exists():
            pid = int(marker.read_text())
            with self.assertRaises(ProcessLookupError):
                os.kill(pid, 0)

    def run_fixture(self, mode):
        script, _ = self.fixture(mode)
        real_popen = subprocess.Popen

        def track(*args, **kwargs):
            proc = real_popen(*args, **kwargs)
            self.processes.append(proc)
            return proc

        with patch.object(smoke_test.subprocess, "Popen", side_effect=track):
            result = smoke_test.run_smoke_test(script, self.plugin, startup_timeout=0.8, grace_timeout=0.3)
        self.assertEqual(len(self.processes), 1)
        self.assert_gone(self.processes[0])
        return result

    def test_normal_lifecycle(self):
        self.assertEqual(self.run_fixture("normal"), 0)

    def test_startup_failure(self):
        self.assertEqual(self.run_fixture("fail"), 1)

    def test_startup_timeout(self):
        self.assertEqual(self.run_fixture("never_ready"), 1)

    def test_ignored_term(self):
        self.assertEqual(self.run_fixture("ignore"), 1)

    def test_orphaned_grandchild(self):
        self.assertEqual(self.run_fixture("grandchild"), 1)

    def test_runner_sigterm(self):
        script, marker = self.fixture("never_ready")
        runner = subprocess.Popen(
            [sys.executable, str(Path(smoke_test.__file__)), "--server", str(script),
             "--plugin", str(self.plugin), "--startup-timeout", "10", "--grace-timeout", "0.3"],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True,
        )
        self.processes.append(runner)
        deadline = time.monotonic() + 3
        while not marker.exists() and runner.poll() is None and time.monotonic() < deadline:
            time.sleep(0.01)
        self.assertTrue(marker.exists(), "server startup handshake must complete")
        server_pid = int(marker.read_text())
        self.addCleanup(lambda: self.kill_group_if_live(server_pid))
        runner.send_signal(signal.SIGTERM)
        self.assertEqual(runner.wait(timeout=5), 143)
        self.assertFalse(smoke_test.is_group_alive(server_pid))
        with self.assertRaises(ProcessLookupError):
            os.kill(server_pid, 0)

    def kill_group_if_live(self, pid):
        try:
            os.killpg(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    def test_group_probe_fails_closed(self):
        with patch.object(os, "killpg", side_effect=PermissionError):
            self.assertTrue(smoke_test.is_group_alive(123))
        with patch.object(os, "killpg", side_effect=ProcessLookupError):
            self.assertFalse(smoke_test.is_group_alive(123))


if __name__ == "__main__":
    unittest.main()

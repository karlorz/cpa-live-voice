#!/usr/bin/env python3
"""Run a bounded CPA plugin smoke test with POSIX process-group cleanup.

SIGTERM, SIGINT, startup failures, and shutdown timeouts trigger cleanup.
SIGKILL of this runner and children that create a new session require an
external process supervisor.
"""

import argparse
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


def find_free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def build_inert_config_yaml(port, plugin_dir, auth_dir):
    return f'''host: "127.0.0.1"
port: {port}
auth-dir: {json.dumps(str(auth_dir))}
api-keys:
  - "cpa-smoke-inert-test-key"
remote-management:
  allow-remote: false
  secret-key: "cpa-smoke-management-key"
  disable-control-panel: true
  disable-auto-update-panel: true
plugins:
  enabled: true
  dir: {json.dumps(str(plugin_dir))}
  configs:
    cpa-live-voice:
      enabled: true
      priority: 100
      live_auth_ids: ["codex-dummy-account"]
      strategy: "fill-first"
      fail_closed: true
'''


def is_group_alive(pgid):
    try:
        os.killpg(pgid, 0)
        return True
    except ProcessLookupError:
        return False
    except OSError:
        return True


def signal_group(pgid, sig):
    try:
        os.killpg(pgid, sig)
    except ProcessLookupError:
        pass


def terminate_process_group(proc, pgid, grace_timeout=5.0, poll_interval=0.05):
    """Reap the child and verify its group is empty; report forced cleanup."""
    signal_group(pgid, signal.SIGTERM)
    deadline = time.monotonic() + grace_timeout
    while proc.poll() is None or is_group_alive(pgid):
        if time.monotonic() >= deadline:
            break
        time.sleep(poll_interval)
    forced = proc.poll() is None or is_group_alive(pgid)
    if forced:
        signal_group(pgid, signal.SIGKILL)
        if proc.poll() is None:
            proc.kill()
        deadline = time.monotonic() + 3.0
        while proc.poll() is None or is_group_alive(pgid):
            if time.monotonic() >= deadline:
                raise RuntimeError(f"process group {pgid} still exists after SIGKILL")
            time.sleep(poll_interval)
    proc.wait(timeout=1.0)
    return not forced and proc.returncode == 0


def check_models_health(port, timeout=0.5):
    request = urllib.request.Request(
        f"http://127.0.0.1:{port}/v1/models",
        headers={"Authorization": "Bearer cpa-smoke-inert-test-key"},
    )
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(request, timeout=timeout) as response:
            return response.status == 200 and isinstance(json.load(response), dict)
    except (OSError, ValueError, urllib.error.URLError):
        return False


def has_plugin_registered_log(log_path):
    return any(
        "plugin registered" in line and "plugin_id=cpa-live-voice" in line
        for line in log_path.read_text(encoding="utf-8", errors="replace").splitlines()
    )


def run_smoke_test(server_path, plugin_path, startup_timeout=15.0, grace_timeout=5.0):
    if os.name != "posix":
        print("Error: this smoke runner requires POSIX process groups", file=sys.stderr)
        return 1
    server_path, plugin_path = Path(server_path).resolve(), Path(plugin_path).resolve()
    if not server_path.is_file() or not plugin_path.is_file():
        print("Error: server and plugin paths must be existing files", file=sys.stderr)
        return 1
    if startup_timeout <= 0 or grace_timeout <= 0:
        print("Error: timeouts must be positive", file=sys.stderr)
        return 1

    interrupted = None

    def on_signal(signum, _frame):
        nonlocal interrupted
        interrupted = signum

    previous = {sig: signal.signal(sig, on_signal) for sig in (signal.SIGTERM, signal.SIGINT)}
    temp_dir = Path(tempfile.mkdtemp(prefix="cpa-smoke-"))
    proc = None
    log_file = None
    result = 1
    healthy = False
    try:
        plugins_dir, auth_dir = temp_dir / "plugins", temp_dir / "auth"
        plugins_dir.mkdir()
        auth_dir.mkdir()
        shutil.copy2(plugin_path, plugins_dir / plugin_path.name)
        port = find_free_port()
        config = temp_dir / "config.yaml"
        config.write_text(build_inert_config_yaml(port, plugins_dir, auth_dir), encoding="utf-8")
        log_path = temp_dir / "server.log"
        log_file = log_path.open("w", encoding="utf-8")
        proc = subprocess.Popen(
            [str(server_path), "--config", str(config), "--local-model", "--no-browser"],
            stdout=log_file, stderr=log_file, cwd=temp_dir, start_new_session=True,
        )
        deadline = time.monotonic() + startup_timeout
        while not interrupted and proc.poll() is None and time.monotonic() < deadline:
            if check_models_health(port) and has_plugin_registered_log(log_path):
                healthy = True
                break
            time.sleep(0.05)
        if not healthy and not interrupted:
            print("Error: server startup or plugin registration failed", file=sys.stderr)
    except Exception as error:
        print(f"Error: {error}", file=sys.stderr)
    finally:
        # Keep signal handling active until process-group cleanup completes.
        try:
            graceful = proc is None or terminate_process_group(proc, proc.pid, grace_timeout)
            if proc is not None and (proc.poll() is None or is_group_alive(proc.pid)):
                raise RuntimeError("process cleanup is incomplete")
            if log_file:
                log_file.close()
            shutil.rmtree(temp_dir)
            result = 128 + interrupted if interrupted else int(not (healthy and graceful))
            if healthy and not graceful:
                print("Error: shutdown required force-kill or returned a nonzero status", file=sys.stderr)
        except Exception as error:
            print(f"Error: cleanup failed: {error}; retained {temp_dir}", file=sys.stderr)
            result = 1
        finally:
            if log_file and not log_file.closed:
                log_file.close()
            for sig, handler in previous.items():
                signal.signal(sig, handler)
    if result == 0:
        print("OK: plugin registered, API healthy, server and process group exited cleanly")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", type=Path, required=True)
    parser.add_argument("--plugin", type=Path, required=True)
    parser.add_argument("--startup-timeout", type=float, default=15.0)
    parser.add_argument("--grace-timeout", type=float, default=5.0)
    args = parser.parse_args()
    return run_smoke_test(args.server, args.plugin, args.startup_timeout, args.grace_timeout)


if __name__ == "__main__":
    sys.exit(main())

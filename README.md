# CPA Live Voice Scheduler Plugin (`cpa-live-voice`)

[![CI](https://github.com/karlorz/cpa-live-voice/actions/workflows/ci.yml/badge.svg)](https://github.com/karlorz/cpa-live-voice/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-0.1.3-blue.svg)](https://github.com/karlorz/cpa-live-voice/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A standalone dynamic C-shared ABI plugin for [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) (CPA).

`cpa-live-voice` acts as the dedicated candidate routing and fail-safe selection engine for OpenAI Realtime / Codex Live Voice (`gpt-live-1-codex`). It enforces a strict operator-configured allowlist of OAuth candidate IDs, offers fill-first or race-safe round-robin selection strategies, provides fail-closed protection, and exposes an embedded web dashboard and diagnostic management routes without modifying upstream CPA.

---

## Key Architecture & Operational Invariants

### 1. Sole Active Scheduler Architecture
- **Sole Scheduler Plugin**: `cpa-live-voice` must be configured with a higher priority than any other scheduler plugin.
- **Normal Traffic Delegation**: For any non-Live request (models other than `gpt-live-1-codex` or providers other than `codex`), `cpa-live-voice` returns a handled delegation response specifying the configured built-in strategy (`fill-first` or `round-robin`). This preserves full Personal Access Token (PAT) eligibility and candidate evaluation via CPA's native candidate pool.
- **Lower-Priority Scheduler Invalidation**: Because `cpa-live-voice` handles all non-Live traffic via internal built-in delegation, lower-priority plugin schedulers in the host chain will not execute.

### 2. Upstream OAuth Candidate Prerequisite
- CPA's core Codex Live and Realtime handlers pre-filter available credentials to OAuth-kind candidates before calling the scheduler. Codex PAT files that carry `access_token` metadata can appear in that pool.
- The plugin registers `scheduler_across_priorities` so Live picks see every available OAuth-kind priority tier, then intersects that set with `live_auth_ids`. A higher-priority PAT therefore cannot hide an allowlisted Plus OAuth credential.
- CPA v8.0.12 currently passes an empty scheduler model on this OAuth-only local path. The plugin recognizes that host call shape, while also accepting the canonical `gpt-live-1-codex` model when a future host passes it explicitly.
- Allowlist matching accepts the host auth ID, its basename, and `path` / `source` attribute basenames.
- Disabled auth files are dropped by CPA before scheduler pick. Host Inventory Check only verifies that the filename exists; it does not mean the credential is enabled.

### 3. Home Deployment Limitation
- Deployments with CLIProxyAPIHome enabled (`home.enabled: true`) bypass local plugin schedulers in CPA core.
- Home-enabled configurations are **unsupported in v0.1**.

### 4. Zero Secrets / Redaction Guarantee
- The plugin never stores, logs, or transmits secrets, tokens, Authorization values, auth-file contents, or SDP offer/answer session payloads.
- Bounded decision history records only timestamps, classification, outcome, aggregate candidate counts, and sanitized reasons. Selected auth IDs are excluded from history and status responses.

---

## Installation

### Via CPA Plugin Store (recommended)

Register this repository as a third-party plugin source under **Plugins → Third-party Plugin Sources** in the CPA management panel:

```
https://raw.githubusercontent.com/karlorz/cpa-live-voice/refs/heads/main/registry.json
```

The store resolves artifacts from this repository's GitHub releases and verifies them against `checksums.txt`.

### Manual install

Download `cpa-live-voice_<version>_<goos>_<goarch>.zip` and `checksums.txt` from [Releases](https://github.com/karlorz/cpa-live-voice/releases), verify the archive against the checksums, and extract the dynamic library into your CPA plugin directory.

## Configuration

Add the plugin configuration under `plugins.configs.cpa-live-voice` in your CPA `config.yaml`:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-live-voice:
      enabled: true
      # Must be higher than any other scheduler plugins
      priority: 100

      # List of authorized OAuth auth IDs for Codex live voice (gpt-live-1-codex)
      live_auth_ids:
        - "codex-account-primary"
        - "codex-account-secondary"

      # Selection strategy across matching live candidates:
      # - "fill-first": first matching candidate in host candidate order (default)
      # - "round-robin": race-safe round-robin rotation across matches
      strategy: "fill-first"

      # Fail-safe mode:
      # - true: reject with "live_oauth_unavailable" if no configured IDs are available (default)
      # - false: delegate un-matched requests to CPA's built-in scheduler
      fail_closed: true
```

Host-managed fields such as `enabled` and `priority` are parsed safely and ignored during plugin internal decoding. Whitespace in `live_auth_ids` is stripped, duplicates and empty strings are removed, and an empty pool is rejected upon configuration.

---

## Management API & Resource Dashboard

### Diagnostic API Endpoints
All management endpoints require the CPA Management Key passed in the `Authorization: Bearer <key>` header:

| Method | Path | Description |
|---|---|---|
| `GET` | `/v0/management/plugins/cpa-live-voice/status` | Returns active plugin configuration, pool metrics, counters, and the bounded recent-decision history (max 32). |
| `POST` | `/v0/management/plugins/cpa-live-voice/validate` | Compares `live_auth_ids` with a candidate ID list (`{"candidates":[{"id":"..."}]}`). The dashboard loads CPA auth-file IDs automatically and does not require a pasted snapshot. Limited to 64 KiB request bodies. |
| `GET` | `/v0/management/plugins/cpa-live-voice/config-example` | Returns configuration template and defaults as JSON and YAML. |

### Browser Resource Dashboard
Access the self-contained dashboard directly in your browser:
```
http://<host>:<port>/v0/resource/plugins/cpa-live-voice/status
```

#### UI Security Features:
- **No External CDNs**: CSS, fonts, and JavaScript are entirely embedded and self-contained.
- **Key Gate**: Prompts for your CPA Management Key and stores it exclusively in `sessionStorage`. It is never stored in `localStorage`, never included in URLs, and never written to browser logs.
- **Read-Only**: Browser configuration-file editing is prohibited to prevent unauthorized runtime mutation.
- **Responsive Theme**: Automatic light/dark styling adhering to system preferences.
- **Audit Table**: Real-time traffic breakdown, pool health indicators, candidate validator, and recent-decision audit log.

---

## Testing and Verification

- Run `go test -race ./...` for the unit and integration suite. The native host-load test runs in a bounded child process; the parent waits for its exit before removing the shared library. Each repeated run gets its own process and runtime threads.
- Run `python3 scripts/test_smoke_test.py` for cleanup regressions covering startup failure, startup timeout, interrupted runners, and children that survive their wrapper or ignore `SIGTERM`. Tests assert the direct child is reaped and its process group is empty.
- On macOS and Linux, run the real server smoke test:

  ```sh
  python3 scripts/smoke_test.py --server /path/to/cli-proxy-api --plugin bin/cpa-live-voice.dylib
  ```

  Use `.so` on Linux. The runner creates an inert temporary configuration, requires successful plugin registration and an HTTP 200 JSON response from `/v1/models`, then verifies a zero-status server exit and an empty process group. Cleanup escalates to `SIGKILL` on timeout and reports test failure. Temporary files are removed after process exit is confirmed.

The runner handles `SIGTERM` and `SIGINT`. An external supervisor is required if the runner itself receives `SIGKILL` or a child deliberately creates a separate session. `--local-model` disables remote model catalog updates; the server may still run its version updater.

## Building and Packaging

### Build Targets
```bash
make test             # Run all unit and integration tests with race detector
make vet              # Run go vet
make fmt              # Run gofmt -s -w
make build            # Compile c-shared dynamic library to bin/cpa-live-voice.<ext>
make package          # Build platform zip to dist/cpa-live-voice_<version>_<os>_<arch>.zip & checksums.txt
make check            # Verify archive layout against CPA store requirements
make clean            # Remove build artifacts
```

### Release Asset Specification
Official release assets conform to the CPA dynamic plugin store format:
```
dist/cpa-live-voice_<version>_<goos>_<goarch>.zip
dist/checksums.txt
```
The root of each `.zip` contains exactly the dynamic library binary (`cpa-live-voice.so`, `cpa-live-voice.dylib`, or `cpa-live-voice.dll`) with no nested directories or extraneous files.

---

## License

This project is licensed under the [MIT License](LICENSE).

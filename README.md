# CPA Live Voice Scheduler Plugin (`cpa-live-voice`)

[![CI](https://github.com/karlorz/cpa-live-voice/actions/workflows/ci.yml/badge.svg)](https://github.com/karlorz/cpa-live-voice/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-0.1.0-blue.svg)](https://github.com/karlorz/cpa-live-voice/releases)
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
- CPA's core Codex Live and Realtime handlers pre-filter available credentials to OAuth-only candidates before calling the scheduler.
- CPA v8.0.12 currently passes an empty scheduler model on this OAuth-only local path. The plugin recognizes that host call shape, while also accepting the canonical `gpt-live-1-codex` model when a future host passes it explicitly.
- `cpa-live-voice` intersects the filtered candidate inventory against `live_auth_ids` and applies the selection rules.

### 3. Home Deployment Limitation
- Deployments with CLIProxyAPIHome enabled (`home.enabled: true`) bypass local plugin schedulers in CPA core.
- Home-enabled configurations are **unsupported in v0.1**.

### 4. Zero Secrets / Redaction Guarantee
- The plugin never stores, logs, or transmits secrets, tokens, Authorization values, auth-file contents, or SDP offer/answer session payloads.
- Bounded decision history records only timestamps, classification, outcome, aggregate candidate counts, and sanitized reasons. Selected auth IDs are excluded from history and status responses.

---

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
| `POST` | `/v0/management/plugins/cpa-live-voice/validate` | Accepts candidate inventory IDs (`{"candidates":[{"id":"..."}]}`) and checks matching/missing pool health without host credential callbacks. Limited to 64 KiB request bodies. |
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

## Building and Packaging

### Build Targets
```bash
make test             # Run all unit and integration tests with race detector
make vet              # Run go vet
make fmt              # Run gofmt -s -w
make build            # Compile c-shared dynamic library to bin/cpa-live-voice.<ext>
make package          # Build platform zip to dist/cpa-live-voice_0.1.0_<os>_<arch>.zip & checksums.txt
make check            # Verify archive layout against CPA store requirements
make clean            # Remove build artifacts
```

### Release Asset Specification
Official release assets conform to the CPA dynamic plugin store format:
```
dist/cpa-live-voice_0.1.0_<goos>_<goarch>.zip
dist/checksums.txt
```
The root of each `.zip` contains exactly the dynamic library binary (`cpa-live-voice.so`, `cpa-live-voice.dylib`, or `cpa-live-voice.dll`) with no nested directories or extraneous files.

---

## License

This project is licensed under the [MIT License](LICENSE).

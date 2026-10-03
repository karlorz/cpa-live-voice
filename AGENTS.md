# AGENTS.md

Dynamic C-shared ABI plugin for CLIProxyAPI (CPA) providing dedicated candidate scheduling and load balancing for Codex Live Voice (`gpt-live-1-codex`).

## Repository Metadata
- GitHub: https://github.com/karlorz/cpa-live-voice
- Plugin ID: `cpa-live-voice`
- Version: `0.1.1`

## Commands
```bash
gofmt -s -w .                 # Format Go source code (required after Go edits)
go vet ./...                  # Vet packages
go test -v -race ./...        # Run unit & integration tests with race detector
make build                    # Build native dynamic library (bin/cpa-live-voice.<ext>)
make package VERSION=0.1.1    # Build and package release zip + checksums.txt
make check                    # Validate release package structure using check_package.py
make clean                    # Remove bin/ and dist/
```

## Architecture & Code Conventions
- `main.go`: Native C-shared ABI plugin entrypoint (`cliproxy_plugin_init`, `cliproxyPluginCall`, `cliproxyPluginFree`, `cliproxyPluginShutdown`).
- `config.go`: Configuration decoder, sanitization, deduplication, and validation logic.
- `classifier.go`: Request classification for canonical `gpt-live-1-codex` requests and CPA's OAuth-only Live call shape with an omitted scheduler model.
- `selector.go`: Allowlist candidate intersection, `fill-first`, `round-robin`, and `fail_closed` / `fail_open` routing.
- `history.go`: Race-safe bounded recent-decision history (max 32 entries) and traffic counters.
- `management.go`: Management API routes (`/status`, `/validate`, `/config-example`) and resource handler.
- `ui.go`: Embedded self-contained HTML/CSS/JS dashboard served at `/v0/resource/plugins/cpa-live-voice/status`.
- `types.go`: Canonical data models and schemas.

## Security & Operational Invariants
- **No Sensitive Leakage**: Never log, store, or emit credentials, tokens, Authorization headers, auth file bodies, or SDP payloads in source, tests, output, or logs.
- **Sole Scheduler Contract**: Must have priority set above other scheduler plugins. Normal non-Live traffic delegates directly to CPA's built-in scheduler, preserving PAT eligibility.
- **SessionStorage Only**: Management Key is stored only in browser `sessionStorage` and sent strictly via `Authorization: Bearer <key>`. It must never appear in URL queries or logs.
- **Docker-Free**: Never use local Docker for builds or tests.

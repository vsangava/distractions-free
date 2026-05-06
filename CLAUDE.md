# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Does

A system-level productivity daemon for macOS (and Windows) that enforces per-time-of-day blocking rules. It runs as a privileged background service (`launchd` on macOS) and offers three interchangeable enforcement modes selected by `enforcement_mode` in config:

- **`hosts`** (default, cross-platform) — rewrites the system hosts file. Survives browser-DoH bypass because `getaddrinfo` reads `/etc/hosts` before any resolver.
- **`dns`** — local DNS proxy on `127.0.0.1:53` returning `0.0.0.0` for blocked domains and forwarding everything else upstream.
- **`strict`** (macOS only, recommended) — DNS proxy plus a `pf` packet-filter anchor that drops outbound packets to the resolved IPs at the kernel. Includes a `_doh` group of public DoH/DoT endpoints that's always-on, so even browsers with a manually-configured DoH provider can't route around the block.

The daemon also closes Chrome/Safari/Arc/Brave tabs via AppleScript on every tick during an active block, fires a 3-minute pre-block notification, and runs a web dashboard on `:8040` for rule editing, status, pause, focus sessions, and quota inspection. All unkillable by regular users.

## Workflow

- Always work in a feature branch, never commit directly to `main`.
- Make granular commits as individual pieces of work are completed — don't batch unrelated changes into one commit.
- When a feature is complete, raise a PR. PR descriptions must be comprehensive: explain **what** was done, **why** it was done that way, and call out any **gotchas** (non-obvious side effects, required manual steps, behavioural changes, or things a reviewer should look out for).

## Commands

```bash
make build          # Build binary for current OS
make build-all      # Cross-compile: macOS arm64, macOS amd64, Windows amd64
make test           # go test ./...
make release        # test + build-all + verify-binaries (pre-release check)
make clean          # Remove built binaries
```

Run a subset of tests:
```bash
go test ./internal/scheduler -v
go test ./internal/proxy -v
```

## Testing Without Root / Service Installation

The binary has four flags for development testing — none require root or service installation:

- `--local` — runs DNS proxy, scheduler, and web UI using local `./config.json` (not system path)
- `--test-query "2024-04-01 10:30" youtube.com` — evaluates blocking at a specific time and queries real upstream DNS
- `--test-web` — starts the web dashboard at `http://localhost:8040`
- `--test-applescript` — generates and optionally executes tab-closing/notification AppleScript

## Architecture

### Data Flow

1. Scheduler ticks every minute, calling `EvaluateRulesAtTime(now, cfg, quotaUsage)` to compute the blocked-domain set. Pause and Pomodoro work-phase are checked first as overrides.
2. The scheduler diffs against the previous tick and hands `newlyBlocked` / `newlyUnblocked` to the active enforcer (one of `HostsEnforcer` / `DNSEnforcer` / `StrictEnforcer`, picked from `enforcement_mode`).
3. On state change: enforcer applies its mode-specific change (`/etc/hosts` rewrite, in-memory blocked map update, or pf anchor regen + DNS cache flush). AppleScript closes matching browser tabs every tick, not just on state transitions.
4. In `strict` mode, the enforcer also calls `pf.Refresh()` every tick to re-resolve CDN IPs that may have rotated since the last activation, keeping the pf table current.
5. The DNS proxy on port 53 (used by `dns` and `strict` modes) checks each query against the `activeBlocks` map and returns `0.0.0.0` if blocked, else forwards to `primary_dns` (default `8.8.8.8:53`) with `backup_dns` failover (default `1.1.1.1:53`). Non-blocked queries for configured groups are appended to a usage log used for daily-quota enforcement.
6. 3 minutes before a block starts, `CheckWarningDomainsAtTime()` triggers native macOS notifications.

### Key Packages

| Package | Responsibility |
|---|---|
| `cmd/app/main.go` | Entry point; dispatches service, test, and direct-run modes; idempotent `setup` and forensic `clean` paths |
| `internal/config` | JSON config loading from OS-specific paths; thread-safe via `RWMutex`; pause/pomodoro state helpers |
| `internal/enforcer` | `Enforcer` interface + factory; `HostsEnforcer` / `DNSEnforcer` / `StrictEnforcer` impls; calls `pf.RemoveAnchorIfPresent` on mode downgrade so leftover state self-heals |
| `internal/pf` | macOS pf anchor management — two-section anchor (regular all-port blocks + DoH/DoT port-restricted blocks on TCP/443 + TCP/UDP/853), multi-resolver IP union, `pfctl` invocations |
| `internal/scheduler` | 1-minute ticker, rule evaluation, pre-block warnings, per-tick AppleScript tab closing, daily quota tracking |
| `internal/proxy` | DNS server; blocking vs. upstream forwarding; primary/backup DNS failover; usage event logging for quotas |
| `internal/web` | HTTP dashboard on `:8040`; embedded static files via `go:embed`; `/api/config`, `/api/status`, `/api/pause`, `/api/pomodoro/*`, `/api/usage`, `/api/test-query` |
| `internal/cleanup` | Per-step idempotent `clean` pipeline — service uninstall, hosts revert, pf anchor removal, DNS reset, config dir delete, installed-binary unlink |
| `internal/testcli` | Shared logic for `--test-query` (CLI) and web UI query handler |

### Testability Pattern

Core logic is extracted into pure functions that accept `time.Time` and `config.Config` rather than calling `time.Now()` or reading global state:
- `scheduler.EvaluateRulesAtTime(t, cfg) map[string]bool`
- `scheduler.CheckWarningDomainsAtTime(t, cfg) []string`
- `proxy.GetDNSResponse(r, blockedDomains, primaryDNS, backupDNS)`

AppleScript execution uses `AppleScriptGenerator` and `ScriptExecutor` interfaces so tests can swap in stubs without macOS.

### Config Location

| OS | Path |
|---|---|
| macOS (service) | `/Library/Application Support/Sentinel/config.json` |
| `--local` | `./config.json` (working directory) |

Config is re-read every scheduler tick — live edits take effect within 60 seconds, no restart needed.

### OS-Specific Notes

- Tab closing and pre-block notifications are macOS-only (AppleScript targets Chrome, Safari, Arc, and Brave). The probe shells out via `su - <console-user> -c osascript ...` because the daemon runs as root.
- DNS cache flush uses `dscacheutil -flushcache` + `killall -HUP mDNSResponder` on macOS.
- `strict` mode is macOS only — `pf` integration is gated behind `//go:build darwin` and the enforcer factory falls back to `dns` mode on other OSes.
- Port 53 binding requires root; service installation requires admin privileges.

## Documentation

When a significant feature is added or changed, evaluate whether each of the following needs updating and propose changes — don't wait to be asked:

- **`docs/index.html`** — the public landing page; update the features grid, FAQ accordion, or any comparison content that's no longer accurate
- **`README.md`** — the primary user guide; update the "What it does" list, FAQ section, platform support table, or any affected configuration/usage sections
- **`DESIGN.md`** — the architecture reference; update data flow, package responsibility table, or any structural decisions that changed
- **`TROUBLESHOOTING.md`** — update or add entries for any new error conditions, config options, or operational behaviours the feature introduces

## Session Log

After merging a PR or completing a significant unit of work, append a new entry to `session-log.md` in the repo root. Follow the existing format in that file:

- **Header:** date, short title describing the work, session ID (first 8 chars of the session), file size and tool-call count if available
- **Opening prompt:** the first substantive thing the user asked for this session
- **What happened:** a narrative of the follow-up instructions and key decisions made during the session
- **Wrap-up:** one or two sentences on what was delivered

Do not wait to be asked — update the file as part of closing out a piece of work, the same way you raise a PR or bump a version.

If multiple Claude Code instances are running in parallel on this repo, each should append its own entry independently. Before appending, re-read the current state of `session-log.md` to avoid overwriting another session's entry that was added concurrently.

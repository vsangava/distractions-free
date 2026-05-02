# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Does

A system-level DNS proxy daemon for macOS (and Windows) that enforces productivity schedules by intercepting DNS at the OS level. It runs as a privileged background service (`launchd` on macOS), blocking distracting domains by returning `0.0.0.0`, closing browser tabs via AppleScript, and sending pre-block warnings — all unkillable by regular users.

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

1. Scheduler ticks every minute, calling `EvaluateRulesAtTime(now, cfg)` to compute `map[string]bool` of blocked domains.
2. On state change: flushes macOS DNS cache, generates AppleScript to close matching browser tabs.
3. DNS proxy on port 53 checks each query against the `activeBlocks` map — returns `0.0.0.0` if blocked, otherwise forwards to upstream (default: 8.8.8.8, fallback: 1.1.1.1).
4. 3 minutes before a block starts, `CheckWarningDomainsAtTime()` triggers native macOS notifications.

### Key Packages

| Package | Responsibility |
|---|---|
| `cmd/app/main.go` | Entry point; dispatches service, test, and direct-run modes |
| `internal/config` | JSON config loading from OS-specific paths; thread-safe via `RWMutex` |
| `internal/scheduler` | 1-minute ticker, rule evaluation, pre-block warnings, AppleScript tab closing |
| `internal/proxy` | DNS server; blocking vs. upstream forwarding; primary/backup DNS failover |
| `internal/web` | HTTP dashboard on `:8040`; embedded static files via `go:embed`; `/api/config` and `/api/test-query` |
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

- Tab closing and pre-block notifications are macOS-only (AppleScript targeting Chrome and Safari).
- DNS cache flush uses `dscacheutil -flushcache` + `killall -HUP mDNSResponder` on macOS.
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

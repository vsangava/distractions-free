# Distractions-Free

A system-level focus daemon for **macOS** (and Windows). It enforces a productivity schedule by blocking distracting domains at the OS layer, closing browser tabs when a block begins, and warning you 3 minutes before. It runs as a privileged background service that ordinary users cannot disable on a whim.

Unlike browser extensions — one click to disable — Distractions-Free puts the block in the operating system itself: `/etc/hosts`, the local DNS resolver, and (in strict mode) the `pf` firewall. To turn it off you have to type your admin password.

---

## Table of contents

**For users**
- [What it does](#what-it-does)
- [How it works](#how-it-works)
- [Platform support](#platform-support)
- [Install](#install)
- [The web dashboard](#the-web-dashboard)
- [Configuration](#configuration)
- [Enforcement modes](#enforcement-modes)
- [Pause & resume](#pause--resume)
- [Uninstall & cleanup](#uninstall--cleanup)
- [Command-line reference](#command-line-reference)

**For developers**
- [Building from source](#building-from-source)
- [Running tests](#running-tests)
- [Test utilities](#test-utilities)
- [Releases](#releases)
- [Architecture](#architecture)

---

## What it does

- **Blocks distracting domains on a schedule** — domains are organised into named groups (e.g. `games`, `social`); each rule binds one group to per-day, per-time-slot windows. The default mode rewrites `/etc/hosts`, so blocking works across every browser, every app, and every network without any client-side configuration.
- **Closes browser tabs when a block starts** — native AppleScript closes matching tabs in Chrome and Safari at the moment the block begins (macOS only).
- **Sends a 3-minute warning** — native macOS notification before a block starts so you can save your work.
- **Auto-reloads config every minute** — edit the config file, save, and changes take effect on the next minute boundary. No restart needed.
- **Survives sleep/wake** — the scheduler is tick-based; locking the lid and reopening hours later works correctly.
- **Bundled web dashboard** — at `http://localhost:8040`, with status, a query-tester, and a PIN-locked management tab. Static assets are embedded in the binary.
- **Forensic uninstall** — `--clean` removes every system change (hosts entries, pf anchor, DNS overrides, service registration, config dir, temp files) in a single command.

## How it works

A single Go binary runs as a privileged service (`launchd` on macOS, Windows Service on Windows). On every minute boundary the **scheduler** evaluates your rules against the current time and produces the set of domains that should be blocked right now. The **enforcer** — selected by `enforcement_mode` in config — applies that set:

| Mode | Mechanism | Port 53? |
|---|---|---|
| `hosts` *(default)* | edits `/etc/hosts` between managed markers | no |
| `dns` | local DNS proxy on `127.0.0.1:53`, returns `0.0.0.0` for blocked domains | yes |
| `strict` | DNS proxy + `pf` firewall (macOS) — blocks the resolved IPs at the kernel | yes |

The enforcer is given the *diff* each tick — domains newly blocked and newly unblocked — so changes are O(diff), not O(rules). The same scheduler also fires the 3-minute warning notification and the AppleScript that closes tabs in Chrome and Safari.

Rule-evaluation logic, DNS-response generation, and hosts-file generation are pure functions that take the current time / config / domain list as arguments. That's how the test suite covers the core behaviour without root, without a real port-53 binding, and without modifying the system.

## Platform support

| Feature | macOS | Windows |
|---|---|---|
| Hosts-file blocking (`hosts` mode, default) | ✅ | ✅ |
| DNS proxy blocking (`dns` mode) | ✅ | ✅ |
| `pf` firewall layer (`strict` mode) | ✅ (DNS-only fallback if pf setup fails) | ❌ |
| Browser tab closing | ✅ Chrome, Safari (AppleScript) | ❌ |
| 3-minute pre-block notifications | ✅ native notifications | ❌ |
| System service / auto-start on boot | ✅ launchd | ✅ Windows Service |
| `--clean` forensic uninstall | ✅ | ✅ |

---

## Install

### 1. Get the binary

**Pre-built (recommended):** download from the latest [GitHub Release](https://github.com/vsangava/distractions-free/releases):

- macOS Apple Silicon: `distractions-free-macos-arm64`
- macOS Intel: `distractions-free-macos-amd64`
- Windows x86_64: `distractions-free-windows-amd64.exe`

Then `chmod +x distractions-free-macos-*` on macOS.

**From source:** see [Building from source](#building-from-source).

### 2. Install and start the service

The service binds privileged resources (writes to `/etc/hosts`, or binds port 53), so installation requires admin rights:

```bash
sudo ./distractions-free install
sudo ./distractions-free start
```

In `hosts` mode (the default) that is everything you need to do — no DNS reconfiguration, no extra steps.

In `dns` or `strict` mode you also need to point your OS resolver at the local proxy. Most users should not bother:

```bash
networksetup -setdnsservers Wi-Fi 127.0.0.1
# or:
networksetup -setdnsservers Ethernet 127.0.0.1
```

### 3. Verify

Open `http://localhost:8040` — the dashboard should load. You should see the seed rule blocking `youtube.com` Mon–Fri 09:00–17:00 (you'll edit this in [Configuration](#configuration)).

---

## The web dashboard

Open **`http://localhost:8040`** while the service is running. The dashboard has three tabs:

| Tab | What it does |
|---|---|
| **Status** | Currently blocked domains, current enforcement mode, paused state, and the next 24 hours of upcoming block/unblock events. |
| **Test** | Evaluate any `(time, domain)` pair against the live rules — useful when designing schedules. Optionally evaluate against a custom config without touching the live one. |
| **Manage** | Edit the live config (rules, settings), trigger a pause, and resume. PIN-protected. |

### Manage-tab PIN

The Manage tab requires a 4-digit PIN before any change is accepted. The PIN is the **current local time as `HHMM`**, in either 24-hour or 12-hour form. It validates client-side and unlocks the tab for the current page session:

| Local time | Valid PINs |
|---|---|
| 2:35 PM | `1435` (24h) or `0235` (12h) |
| 9:05 AM | `0905` |
| 9:05 PM | `2105` (24h) or `0905` (12h) |
| 12:00 noon | `1200` |
| 12:00 midnight | `0000` (24h) or `1200` (12h) |

The PIN is a friction layer, not a security control. The server-side auth is the **auth token** stored in `config.json` under `settings.auth_token`. The web UI fetches it from the public `GET /api/config` endpoint on first load and sends it in the `X-Auth-Token` header for every other API call. If you don't trust local users on the machine, treat the auth token as a secret — the dashboard listens only on `127.0.0.1`, but anything running locally as your user can read it.

---

## Configuration

The daemon reads its configuration from a single JSON file:

| OS | Path |
|---|---|
| macOS (service) | `/Library/Application Support/DistractionsFree/config.json` |
| Windows (service) | `%PROGRAMDATA%\DistractionsFree\config.json` |
| Linux | `/etc/distractionsfree/config.json` |
| Any (`--no-service`) | `./config.json` (working directory) |

The file is generated with sensible defaults on first launch. The scheduler reloads it every minute, so live edits take effect on the next tick — no restart required.

### Example

```json
{
  "settings": {
    "primary_dns": "8.8.8.8:53",
    "backup_dns": "1.1.1.1:53",
    "enforcement_mode": "hosts",
    "auth_token": "c911284368ac967797e8af4379b3bcb6"
  },
  "groups": {
    "games":  ["roblox.com", "epicgames.com", "steampowered.com", "fortnite.com", "minecraft.net"],
    "social": ["discord.com", "facebook.com", "instagram.com", "tiktok.com", "snapchat.com", "reddit.com"]
  },
  "rules": [
    {
      "group": "games",
      "is_active": true,
      "schedules": {
        "Monday":    [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Tuesday":   [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Wednesday": [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Thursday":  [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Friday":    [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Saturday":  [{"start": "21:30", "end": "23:59"}],
        "Sunday":    [{"start": "21:30", "end": "23:59"}]
      }
    },
    {
      "group": "social",
      "is_active": true,
      "schedules": {
        "Monday":    [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Tuesday":   [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Wednesday": [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Thursday":  [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Friday":    [{"start": "09:00", "end": "15:00"}, {"start": "21:30", "end": "23:59"}],
        "Saturday":  [{"start": "21:30", "end": "23:59"}],
        "Sunday":    [{"start": "21:30", "end": "23:59"}]
      }
    }
  ]
}
```

The default seed above blocks both groups during school hours (9–3 weekdays) plus every night from 9:30pm — adjust the slots, drop a group from `rules`, or define your own (`work`, `news`, `streaming`…) as needed.

### Field reference

- **`settings.primary_dns` / `backup_dns`** — upstream resolvers for `dns`/`strict` mode. Ignored in `hosts` mode.
- **`settings.enforcement_mode`** — `"hosts"` (default), `"dns"`, or `"strict"`. See [Enforcement modes](#enforcement-modes).
- **`settings.auth_token`** — auto-generated 32-char hex on first launch. The web UI requires this token for every API call except `GET /api/config` (which is intentionally public so the UI can bootstrap the token).
- **`groups`** — named lists of bare domains (no `www.`) that schedules are bound to. Edit a group once and every rule that uses it updates. In `hosts` mode the enforcer automatically also blocks the prefixes `www.`, `m.`, `mobile.`, `app.` for each member; in `dns` mode subdomain matching is suffix-based (`a.b.example.com` is blocked if `example.com` is in the group).
- **`rules[].group`** — the group name this rule schedules. Must match a key in `groups`. Validation rejects unknown references.
- **`rules[].is_active`** — set to `false` to suspend a single rule without deleting it.
- **`rules[].schedules`** — object keyed by weekday name (`"Monday"` … `"Sunday"`). Each value is an array of `{start, end}` slots in `HH:MM` 24-hour format. Every domain in the rule's group is blocked when current time falls in `[start, end)`.
- **`pause`** *(optional)* — `{"until": "<RFC3339 timestamp>"}`. While set, all blocking is suspended. Cleared automatically once `until` passes. Easiest to set via the dashboard or the [`/api/pause`](#http-api) endpoint.

### Editing rules from the command line

You can edit the config file directly with any editor — the daemon picks up changes within 60 seconds. It must be valid JSON; if parsing fails, the daemon logs a warning and keeps using the previous in-memory copy.

---

## Enforcement modes

The default is `"hosts"`. Most users should leave it that way. Choose another mode only if you have a specific reason.

### `hosts` (default)

Edits `/etc/hosts` (macOS/Linux) or `C:\Windows\System32\drivers\etc\hosts` (Windows). Blocked entries live between marker lines (`# distractions-free:begin` / `# distractions-free:end`) and are atomically rewritten via temp file + rename, so a crash mid-write cannot corrupt the file. Other entries are never touched.

**Pros:** no port binding, works in every browser and every app, survives DNS-server changes, no system DNS reconfiguration.

**Cons:** no wildcards (only the static prefix list), so very deep CDN domains are not covered.

### `dns`

Runs a local DNS proxy on `127.0.0.1:53`. Blocked domains return `0.0.0.0`; everything else is forwarded to `primary_dns` (failover to `backup_dns`). On macOS the daemon also resets system DNS on shutdown.

**Pros:** suffix-matched subdomain blocking (`a.b.example.com` blocked if `example.com` is in the list).

**Cons:** requires you to point your OS DNS at `127.0.0.1`. Apps that hard-code their own DNS resolver (some VPN clients, some browsers in DoH mode) bypass it.

### `strict`

Like `dns`, plus a `pf` (Packet Filter) anchor on macOS that drops outbound packets to the resolved IPs at the kernel level. Each tick, the enforcer resolves blocked domains to A/AAAA addresses and rewrites the pf anchor table. Existing connections to those IPs are killed.

**Pros:** harder to bypass than DNS alone — even apps with their own resolver can't reach the IPs.

**Cons:** macOS only. CDN-heavy domains rotate IPs and only the resolved subset is blocked. If pf setup fails (permissions, edited `pf.conf`), the enforcer logs a warning and degrades to DNS-only.

### Switching modes

Either edit `enforcement_mode` in `config.json` and restart the service, or use the shorthand:

```bash
sudo ./distractions-free --strict   # writes "strict" to config and exits
sudo ./distractions-free start      # restart to pick it up
```

---

## Pause & resume

Need to break the rules — interview, demo, watching a YouTube link a colleague sent? Use the **Manage** tab to pause for up to 4 hours, or hit the API directly:

```bash
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')

# Pause for 30 minutes
curl -X POST http://localhost:8040/api/pause \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"minutes": 30}'

# Resume immediately
curl -X DELETE http://localhost:8040/api/pause \
  -H "X-Auth-Token: $TOKEN"
```

A pause window is persisted in `config.json` as `pause.until`. The scheduler clears it automatically once the timestamp passes, so you don't end up with stale pauses lying around.

---

## Uninstall & cleanup

The recommended path is the all-in-one `--clean` command. It stops the service, removes hosts entries, removes the pf anchor, resets DNS on every interface that was pointing at `127.0.0.1`, flushes the resolver cache, uninstalls the service registration, removes the config directory, deletes temp files, and verifies port 53 is free:

```bash
sudo ./distractions-free --clean          # interactive: asks before deleting config
sudo ./distractions-free --clean --yes    # non-interactive: deletes config without asking
```

If you'd rather drive the steps yourself:

```bash
sudo ./distractions-free stop        # restores DNS in dns/strict mode; clears hosts entries in hosts mode
sudo ./distractions-free uninstall   # removes the service registration
sudo rm -rf "/Library/Application Support/DistractionsFree"
```

> ⚠️ In `dns`/`strict` mode, do not delete the binary while the service is running with system DNS pointed at `127.0.0.1`, or the machine will lose name resolution. `--clean` handles this correctly; manual removal does not.

---

## Command-line reference

```
distractions-free <subcommand>           # service management (kardianos/service)
distractions-free [--flag]               # local / test mode
sudo distractions-free --clean [--yes]   # forensic uninstall
```

| Command / flag | Privileges | What it does | More |
|---|---|---|---|
| `install` | sudo | Register the system service (launchd / Windows Service) | [Install](#install) |
| `uninstall` | sudo | Remove the service registration | [Uninstall](#uninstall--cleanup) |
| `start` | sudo | Start the service in the background | [Install](#install) |
| `stop` | sudo | Stop the service; clears hosts entries / restores DNS as appropriate | [Uninstall](#uninstall--cleanup) |
| `status` | sudo | Print whether the service is running | — |
| `run` | sudo | Run as if launched by the service supervisor (foreground) | — |
| `--no-service` | none | Run the daemon in the foreground using `./config.json` (skips system paths). In `dns`/`strict` mode you still need root to bind port 53. | [Test utilities](#test-utilities) |
| `--strict` | sudo | Set `enforcement_mode` to `"strict"` in config and exit. Restart the service to apply. | [Switching modes](#switching-modes) |
| `--clean [--yes]` | sudo | Forensic recovery: undo every system change. Use this before deleting the binary. | [Uninstall](#uninstall--cleanup) |
| `--test-query "<YYYY-MM-DD HH:MM>" <domain>` | none | Print whether a domain would be blocked at a specific time, with the matching rules. Uses `./config.json`. | [Test utilities](#test-utilities) |
| `--test-web` | none | Start the web dashboard standalone for testing schedules without installing the service. Uses `./config.json`. | [Test utilities](#test-utilities) |
| `--test-applescript` | none | Generate the AppleScript that closes Chrome/Safari tabs and optionally execute it. macOS only. | [Test utilities](#test-utilities) |

### HTTP API

All endpoints listen on `127.0.0.1:8040`. Every endpoint except the first requires `X-Auth-Token: <auth_token>` (read it from `GET /api/config`).

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/config` | GET | Current config (public — used to bootstrap the token in the UI) |
| `/api/config/update` | POST | Replace the rules / settings; validated server-side |
| `/api/status` | GET / POST | Current blocked domains, last evaluation, paused state |
| `/api/test-query?time=&domain=` | GET / POST | Evaluate `(time, domain)`; POST a `config` form field to test against a custom config |
| `/api/hosts-preview` | GET / POST | Show the `/etc/hosts` lines that would be written for the currently blocked set |
| `/api/pf-preview` | GET / POST | Show the resolved IPs and `pf` anchor content (strict mode only) |
| `/api/pause` | POST | Body `{"minutes": N}` (1–240) |
| `/api/pause` | DELETE | Resume immediately |

---

# For developers

## Building from source

Requires Go 1.21+ (the release pipeline uses 1.26.2).

```bash
git clone https://github.com/vsangava/distractions-free.git
cd distractions-free

make build         # current OS only
make build-all     # macOS arm64 + amd64 + Windows amd64
```

`make help` lists every target. The full list:

| Target | What it does |
|---|---|
| `make build` | Build for current OS into `./distractions-free` |
| `make build-all` | Cross-compile macOS arm64, macOS amd64, and Windows amd64 |
| `make test` | `go test ./...` |
| `make release` | `test` + `build-all` + `verify-binaries` (pre-release sanity check) |
| `make clean` | Remove built binaries |

## Running tests

```bash
go test ./...                       # everything
go test ./internal/scheduler -v     # rule evaluation
go test ./internal/proxy -v         # DNS response generation
go test ./internal/enforcer -v      # hosts/dns/strict enforcer logic
go test ./internal/web -v           # HTTP handlers
```

The whole suite runs without root, without binding port 53, and without modifying any system file. Core logic is implemented as pure functions (`scheduler.EvaluateRulesAtTime`, `proxy.GetDNSResponse`, `enforcer.GenerateHostsEntries`, `pf.GenerateAnchorContent`) so tests pass `time.Time`, `config.Config`, and domain lists in directly.

The DNS-response tests query real upstream resolvers (`8.8.8.8`, `1.1.1.1`) to verify forwarding and failover, so you'll need internet access for `./internal/proxy`.

What the test suite does **not** cover (these are validated by hand because they need root and a live macOS environment):

- Port 53 binding
- System DNS reconfiguration (`networksetup`)
- `pf` rule loading
- AppleScript tab-closing in Chrome/Safari
- Service registration in launchd / Windows Service Manager

## Test utilities

Three flags let you exercise the daemon without installing it:

### `--test-query "<time>" <domain>`

```bash
./distractions-free --test-query "2024-04-01 10:30" youtube.com
```

Prints whether the domain would be blocked at that moment, the matching rule(s), and whether a 3-minute warning would fire. Uses `./config.json`. Time format: `YYYY-MM-DD HH:MM` (24-hour).

### `--test-web`

```bash
./distractions-free --test-web
# open http://localhost:8040
```

Runs the full dashboard against `./config.json` without installing the service. The Manage tab can edit the local config; nothing system-level is touched. The hosts-preview and pf-preview endpoints are particularly useful here — they show exactly what *would* be written to `/etc/hosts` or loaded into pf, without root.

### `--test-applescript`

```bash
./distractions-free --test-applescript
```

Generates the AppleScript used by the tab-closer and optionally executes it. macOS only. Test domains: `facebook.com` (close), `reddit.com` and `roblox.com` (warning — only fires if a tab is open).

## Releases

Tagged commits matching `v*` trigger the GitHub Actions workflow at `.github/workflows/release.yml`:

```bash
make release          # local sanity check: tests + build-all
git tag v1.2.3
git push origin v1.2.3
```

The workflow builds for `darwin/arm64` and `windows/amd64`, runs `go test ./...` on each, and attaches the binaries to the release. macOS Intel (`darwin/amd64`) is built locally by `make build-all` but is **not** part of the release matrix at the moment — add a row to `.github/workflows/release.yml` if you need it on the release page.

The release matrix uses Go 1.26.2 with `CGO_ENABLED=0` so the resulting binaries are statically linked.

## Architecture

For a deep dive into modules, data flow, and the enforcer abstraction, see [DESIGN.md](./DESIGN.md). For diagnostic and recovery procedures, see [TROUBLESHOOTING.md](./TROUBLESHOOTING.md).

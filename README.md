# Sentinel

*Your schedule. Enforced.*

[![Latest Release](https://img.shields.io/github/v/release/vsangava/sentinel?label=release)](https://github.com/vsangava/sentinel/releases/latest)
[![Release Date](https://img.shields.io/github/release-date/vsangava/sentinel)](https://github.com/vsangava/sentinel/releases/latest)
[![Build](https://img.shields.io/github/actions/workflow/status/vsangava/sentinel/ci.yml?branch=main&label=build)](https://github.com/vsangava/sentinel/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows-lightgrey)](https://github.com/vsangava/sentinel/releases/latest)

Sentinel lets you set a schedule for the websites that are hardest to resist — social media, gaming, streaming — and enforces it without giving you (or anyone else) an easy way out. It runs silently in the background as a system service. Turning it off requires your admin password.

It works across every browser and every app, not just the one you happen to have open. There's no extension to disable, no toggle to flick.

## Why not a browser extension?

Browser extensions are easy to bypass — one click to disable, and kids know it. Sentinel works at the operating system level: it rewrites the system hosts file (or runs a local DNS resolver in advanced mode), so blocked sites don't load anywhere on the machine, in any browser or app. Getting around it requires admin credentials.

---

<!-- Screenshot: dashboard Status tab — coming soon -->

---

## Table of contents

- [What it does](#what-it-does)
- [Platform support](#platform-support)
- [Install](#install)
- [The web dashboard](#the-web-dashboard)
- [Configuration](#configuration)
- [Pause & resume](#pause--resume)
- [Enforcement modes](#enforcement-modes)
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

- **Set a schedule, not a willpower battle** — group the sites you want to limit (e.g. `games`, `social`) and define exactly when they're off-limits. Blocks apply the moment the clock hits your window, and lift the moment it ends.
- **Tabs close automatically** — when a block begins, Chrome and Safari close any open tabs for blocked sites. No willpower required. (macOS)
- **A heads-up before the block** — a native notification appears 3 minutes early so you can finish what you're doing before the sites go dark. (macOS)
- **Live config — no restart needed** — edit the schedule file and save; changes take effect within 60 seconds.
- **Built-in web dashboard** — manage rules, check what's currently blocked, and test schedules before they go live, all at `http://localhost:8040`.
- **Survives sleep and wake** — closing the lid and reopening hours later works correctly; the scheduler catches up on the next minute tick.
- **Clean uninstall** — one command removes every system change the daemon made, leaving the machine exactly as it was.

## Platform support

| Feature | macOS | Windows |
|---|---|---|
| Hosts-file blocking (default mode) | ✅ | ✅ |
| DNS proxy blocking (advanced mode) | ✅ | ✅ |
| Firewall-layer blocking (strict mode) | ✅ | ❌ |
| Browser tab closing | ✅ Chrome, Safari | ❌ |
| 3-minute pre-block notifications | ✅ | ❌ |
| Auto-start on boot | ✅ | ✅ |
| One-command clean uninstall | ✅ | ✅ |

---

## Install

### Option A — One-liner (recommended)

```bash
curl -fsSL https://github.com/vsangava/sentinel/releases/latest/download/install.sh | sudo bash
```

The script detects your Mac's architecture, downloads the right binary, removes the macOS quarantine flag, installs it to `/usr/local/bin/sentinel`, registers the service, and starts it.

Prefer to inspect before running?

```bash
curl -fsSL https://github.com/vsangava/sentinel/releases/latest/download/install.sh -o install.sh
less install.sh
sudo bash install.sh
```

### Option B — Manual (two commands)

**[→ Download from the latest release](https://github.com/vsangava/sentinel/releases/latest)**

| Platform | File |
|---|---|
| macOS Apple Silicon | `sentinel-macos-arm64` |
| macOS Intel | `sentinel-macos-amd64` |
| Windows x86_64 | `sentinel-windows-amd64.exe` |

```bash
chmod +x sentinel-macos-arm64          # or sentinel-macos-amd64 on Intel
sudo ./sentinel-macos-arm64 setup
```

`setup` copies the binary to `/usr/local/bin/sentinel`, registers the service with launchd, and starts it. You can delete the downloaded file afterwards.

### Verify

Open `http://localhost:8040` — the dashboard should load. It comes pre-loaded with sample rules for `games`, `videos`, and `social` groups. Edit the config to match your needs ([Configuration](#configuration)).

<details>
<summary>Advanced: DNS and strict modes</summary>

In `dns` or `strict` mode you also need to point your OS resolver at the local proxy:

```bash
networksetup -setdnsservers Wi-Fi 127.0.0.1
# or:
networksetup -setdnsservers Ethernet 127.0.0.1
```

Most users should leave the default `hosts` mode and skip this step. See [Enforcement modes](#enforcement-modes) for the differences.
</details>

---

## The web dashboard

Open **`http://localhost:8040`** while the service is running.

**Status tab** shows what's blocked right now, the current enforcement mode, and a timeline of upcoming block and unblock events for the next 24 hours. A good way to sanity-check that your schedule is doing what you expect.

<!-- Screenshot: dashboard Status tab — coming soon -->

**Test tab** lets you ask "would this site be blocked at this time?" before committing to a schedule. You can test against the live rules or paste in a draft config without touching anything live.

**Manage tab** is where you edit rules, add domain groups, and adjust settings. It's PIN-protected to add a small hurdle against impulsive changes — the PIN is just the current time (`HHMM`). The server also enforces an auth token for every API write, so the PIN isn't your only protection.

<!-- Screenshot: dashboard Manage tab — coming soon -->

### About the PIN

The PIN is the **current local time in `HHMM` format**, in 24-hour or 12-hour form:

| Local time | Valid PINs |
|---|---|
| 2:35 PM | `1435` or `0235` |
| 9:05 AM | `0905` |
| 9:05 PM | `2105` or `0905` |
| 12:00 noon | `1200` |
| 12:00 midnight | `0000` or `1200` |

The PIN is a friction layer — a moment of pause before making changes — not a security boundary. The real protection is the auth token in `config.json`, which the web UI uses for every mutating API call.

---

## Configuration

The daemon reads its configuration from a single JSON file:

| OS | Path |
|---|---|
| macOS (service) | `/Library/Application Support/Sentinel/config.json` |
| Windows (service) | `%PROGRAMDATA%\Sentinel\config.json` |
| Any (`--local`) | `./config.json` (working directory) |

The file is created with defaults on first launch. The scheduler reloads it every minute — live edits take effect on the next tick, no restart required.

### Example config

```json
{
  "settings": {
    "primary_dns": "8.8.8.8:53",
    "backup_dns": "1.1.1.1:53",
    "enforcement_mode": "hosts",
    "auth_token": "<auto-generated on first launch>"
  },
  "groups": {
    "games":  ["roblox.com", "rbxcdn.com", "epicgames.com", "steampowered.com", "fortnite.com", "minecraft.net"],
    "videos": ["youtube.com", "youtu.be", "twitch.tv", "netflix.com", "hulu.com", "primevideo.com", "disneyplus.com", "vimeo.com", "dailymotion.com"],
    "social": ["discord.com", "facebook.com", "instagram.com", "tiktok.com", "snapchat.com", "reddit.com"]
  },
  "rules": [
    {
      "group": "games",
      "is_active": true,
      "schedules": {
        "Monday":    [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Tuesday":   [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Wednesday": [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Thursday":  [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Friday":    [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Saturday":  [{"start": "09:00", "end": "12:00"}, {"start": "18:00", "end": "19:00"}],
        "Sunday":    [{"start": "09:00", "end": "12:00"}, {"start": "18:00", "end": "19:00"}]
      }
    },
    {
      "group": "videos",
      "is_active": true,
      "schedules": {
        "Monday":    [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Tuesday":   [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Wednesday": [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Thursday":  [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Friday":    [{"start": "09:00", "end": "12:00"}, {"start": "14:00", "end": "16:00"}, {"start": "18:00", "end": "19:00"}],
        "Saturday":  [{"start": "09:00", "end": "12:00"}, {"start": "18:00", "end": "19:00"}],
        "Sunday":    [{"start": "09:00", "end": "12:00"}, {"start": "18:00", "end": "19:00"}]
      }
    },
    {
      "group": "social",
      "is_active": true,
      "schedules": {
        "Monday":    [{"start": "00:00", "end": "23:59"}],
        "Tuesday":   [{"start": "00:00", "end": "23:59"}],
        "Wednesday": [{"start": "00:00", "end": "23:59"}],
        "Thursday":  [{"start": "00:00", "end": "23:59"}],
        "Friday":    [{"start": "00:00", "end": "23:59"}],
        "Saturday":  [{"start": "00:00", "end": "23:59"}],
        "Sunday":    [{"start": "00:00", "end": "23:59"}]
      }
    }
  ]
}
```

The default config ships with three groups. `games` and `videos` are blocked during focus windows — mornings (9–12), afternoons (2–4), and an evening hour (6–7) on weekdays; mornings and the evening hour on weekends. `social` is blocked all day, every day. Adjust the time slots, add your own groups (`work`, `news`), or set `is_active: false` on a rule to suspend it without deleting it.

### Field reference

- **`settings.enforcement_mode`** — `"hosts"` (default), `"dns"`, or `"strict"`. See [Enforcement modes](#enforcement-modes).
- **`settings.primary_dns` / `backup_dns`** — upstream resolvers used in `dns`/`strict` mode. Ignored in `hosts` mode.
- **`settings.auth_token`** — auto-generated on first launch. The web UI sends this in `X-Auth-Token` for every mutating API call.
- **`groups`** — named lists of domains that rules are bound to. In `hosts` mode, common prefixes (`www.`, `m.`, `mobile.`, `app.`) are blocked automatically. In `dns` mode, subdomain matching is suffix-based (`a.b.example.com` is blocked if `example.com` is in the group).
- **`rules[].group`** — must match a key in `groups`.
- **`rules[].is_active`** — set to `false` to suspend a rule without deleting it.
- **`rules[].schedules`** — keyed by weekday (`"Monday"` … `"Sunday"`). Each value is an array of `{start, end}` slots in `HH:MM` 24-hour format. Domains are blocked when the current time falls in `[start, end)`.
- **`pause`** *(optional)* — `{"until": "<RFC3339 timestamp>"}`. All blocking suspended until that time. Cleared automatically when the timestamp passes.

---

## Pause & resume

Sometimes you need a legitimate break from your own rules — a video call, a quick research rabbit hole, a link a colleague sent. Use the **Manage** tab in the dashboard to pause for up to 4 hours, or use the API:

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

The pause window is written to `config.json` as `pause.until` and cleared automatically when the time passes — no stale pauses left behind.

---

## Enforcement modes

The default is `"hosts"`. Most users should leave it that way.

### `hosts` (default)

Edits `/etc/hosts` (macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows). Blocked entries live between marker lines (`# sentinel:begin` / `# sentinel:end`) and are atomically rewritten — a crash mid-write cannot corrupt the file. Other entries are never touched.

**Best for:** most users. No port binding, no DNS reconfiguration, works in every browser and app.

**Limitation:** no wildcards — only a static prefix list (`www.`, `m.`, `mobile.`, `app.`), so very deep CDN subdomains aren't covered.

### `dns`

Runs a local DNS proxy on `127.0.0.1:53`. Blocked domains return `0.0.0.0`; everything else forwards to `primary_dns` (failover to `backup_dns`).

**Best for:** users who need wildcard subdomain blocking — any `*.example.com` subdomain is blocked if `example.com` is in the group.

**Requires:** pointing your OS DNS at `127.0.0.1` (see [Install](#install) advanced note). Apps that hard-code their own resolver (some VPNs, some browsers in DoH mode) bypass it.

### `strict`

Like `dns`, plus a `pf` (Packet Filter) anchor on macOS that drops outbound packets to the resolved IPs at the kernel level. Each tick the enforcer resolves blocked domains to A/AAAA addresses and rewrites the firewall table.

**Best for:** situations where DNS-level blocking isn't enough — e.g., apps that ignore the system resolver entirely.

**macOS only.** CDN-heavy sites rotate IPs, so only the resolved subset is blocked. If pf setup fails, the enforcer degrades to DNS-only and logs a warning.

### Switching modes

Edit `enforcement_mode` in `config.json` and restart the service, or use the shorthand:

```bash
sudo ./sentinel --set-mode strict   # writes "strict" to config and exits
sudo ./sentinel start               # restart to pick it up
```

---

## Uninstall & cleanup

The all-in-one `clean` command undoes every system change the daemon made: stops the service, removes hosts entries, removes the pf anchor, resets DNS on every interface pointing at `127.0.0.1`, flushes the resolver cache, unregisters the service, removes the config directory, and verifies port 53 is free.

```bash
sudo sentinel clean         # asks before deleting config
sudo sentinel clean --yes   # deletes config without asking
```

If you'd rather drive the steps yourself:

```bash
sudo ./sentinel stop        # restores DNS/hosts changes
sudo ./sentinel uninstall   # removes the service registration
sudo rm -rf "/Library/Application Support/Sentinel"
```

> ⚠️ In `dns`/`strict` mode, do not delete the binary while the service is running with system DNS pointed at `127.0.0.1`, or the machine will lose name resolution. `clean` handles this correctly; manual removal does not.

---

## Command-line reference

```
sentinel <subcommand>           # service management
sentinel [--flag]               # local / test mode
sudo sentinel setup             # install + start in one command
sudo sentinel clean [--yes]     # forensic uninstall
```

| Command / flag | Privileges | What it does | More |
|---|---|---|---|
| `setup` | sudo | Copy binary to `/usr/local/bin/sentinel`, register service, and start it (macOS: copies; Windows: install+start only) | [Install](#install) |
| `install` | sudo | Register the system service (launchd / Windows Service) | [Install](#install) |
| `uninstall` | sudo | Remove the service registration | [Uninstall](#uninstall--cleanup) |
| `start` | sudo | Start the service in the background | [Install](#install) |
| `stop` | sudo | Stop the service; clears hosts entries / restores DNS | [Uninstall](#uninstall--cleanup) |
| `status` | sudo | Print whether the service is running | — |
| `run` | sudo | Run as if launched by the service supervisor (foreground) | — |
| `clean [--yes]` | sudo | Undo every system change; use before deleting the binary | [Uninstall](#uninstall--cleanup) |
| `--local` | none | Run the daemon in the foreground using `./config.json` | [Test utilities](#test-utilities) |
| `--set-mode <mode>` | sudo | Set `enforcement_mode` in config and exit (modes: `hosts`, `dns`, `strict`) | [Switching modes](#switching-modes) |
| `--test-query "<YYYY-MM-DD HH:MM>" <domain>` | none | Check whether a domain would be blocked at a specific time | [Test utilities](#test-utilities) |
| `--test-web` | none | Start the dashboard standalone without installing the service | [Test utilities](#test-utilities) |
| `--test-applescript` | none | Generate and optionally run the tab-closing AppleScript (macOS) | [Test utilities](#test-utilities) |

### HTTP API

All endpoints listen on `127.0.0.1:8040`. Every endpoint except the first requires `X-Auth-Token: <auth_token>`.

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/config` | GET | Current config (public — used to bootstrap the token in the UI) |
| `/api/config/update` | POST | Replace the rules / settings; validated server-side |
| `/api/status` | GET / POST | Current blocked domains, last evaluation, paused state |
| `/api/test-query?time=&domain=` | GET / POST | Evaluate `(time, domain)`; POST a `config` field to test a custom config |
| `/api/hosts-preview` | GET / POST | Show the `/etc/hosts` lines that would be written for the current blocked set |
| `/api/pf-preview` | GET / POST | Show the resolved IPs and `pf` anchor content (strict mode only) |
| `/api/pause` | POST | Body `{"minutes": N}` (1–240) |
| `/api/pause` | DELETE | Resume immediately |

---

# For developers

## Building from source

Requires Go 1.21+ (the release pipeline uses 1.26.2).

```bash
git clone https://github.com/vsangava/sentinel.git
cd sentinel

make build         # current OS only
make build-all     # macOS arm64 + amd64 + Windows amd64
```

| Target | What it does |
|---|---|
| `make build` | Build for current OS into `./sentinel` |
| `make build-all` | Cross-compile macOS arm64, macOS amd64, and Windows amd64 |
| `make test` | `go test ./...` |
| `make release` | `test` + `build-all` + `verify-binaries` (pre-release sanity check) |
| `make clean` | Remove built binaries |
| `make dev-install` | Build and install the service locally (`build` + `setup`); requires sudo |
| `make dev-uninstall` | Uninstall and fully clean up the service; requires sudo |

## Running tests

```bash
go test ./...                       # everything
go test ./internal/scheduler -v     # rule evaluation
go test ./internal/proxy -v         # DNS response generation
go test ./internal/enforcer -v      # hosts/dns/strict enforcer logic
go test ./internal/web -v           # HTTP handlers
```

The whole suite runs without root, without binding port 53, and without modifying any system file. Core logic is implemented as pure functions (`scheduler.EvaluateRulesAtTime`, `proxy.GetDNSResponse`, `enforcer.GenerateHostsEntries`, `pf.GenerateAnchorContent`) so tests pass `time.Time`, `config.Config`, and domain lists in directly.

The DNS-response tests query real upstream resolvers (`8.8.8.8`, `1.1.1.1`), so you'll need internet access for `./internal/proxy`.

What the test suite does **not** cover (validated by hand — requires root and a live macOS environment):

- Port 53 binding
- System DNS reconfiguration (`networksetup`)
- `pf` rule loading
- AppleScript tab-closing in Chrome/Safari
- Service registration in launchd / Windows Service Manager

## Test utilities

Three flags let you exercise the daemon without installing it as a service. (`--local` is a fourth — it runs the full daemon using `./config.json`.)

### `--test-query "<time>" <domain>`

```bash
./sentinel --test-query "2024-04-01 10:30" youtube.com
```

Prints whether the domain would be blocked at that moment, the matching rule(s), and whether a 3-minute warning would fire. Uses `./config.json`. Time format: `YYYY-MM-DD HH:MM` (24-hour).

### `--test-web`

```bash
./sentinel --test-web
# open http://localhost:8040
```

Runs the full dashboard against `./config.json` without installing the service. The hosts-preview and pf-preview endpoints show exactly what *would* be written without needing root.

### `--test-applescript`

```bash
./sentinel --test-applescript
```

Generates the AppleScript used by the tab-closer and optionally executes it. macOS only. Test domains: `facebook.com` (close), `reddit.com` and `roblox.com` (warning — only fires if a tab is open).

## Releases

Tagged commits matching `v*` trigger the GitHub Actions workflow at `.github/workflows/release.yml`:

```bash
make release          # local sanity check: tests + build-all
git tag v1.2.3
git push origin v1.2.3
```

The workflow builds for `darwin/arm64` and `windows/amd64`, runs `go test ./...` on each, and attaches the binaries to the release. macOS Intel (`darwin/amd64`) is built locally by `make build-all` but is not part of the release matrix — add a row to `.github/workflows/release.yml` if you need it on the release page.

## Architecture

For a deep dive into modules, data flow, and the enforcer abstraction, see [DESIGN.md](./DESIGN.md). For diagnostic and recovery procedures, see [TROUBLESHOOTING.md](./TROUBLESHOOTING.md).

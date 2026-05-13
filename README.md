# Sentinel

*Your schedule. Enforced.*

[![Latest Release](https://img.shields.io/github/v/release/vsangava/sentinel?label=release)](https://github.com/vsangava/sentinel/releases/latest)
[![Release Date](https://img.shields.io/github/release-date/vsangava/sentinel)](https://github.com/vsangava/sentinel/releases/latest)
[![Build](https://img.shields.io/github/actions/workflow/status/vsangava/sentinel/ci.yml?branch=main&label=build)](https://github.com/vsangava/sentinel/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows-lightgrey)](https://github.com/vsangava/sentinel/releases/latest)

Sentinel lets you set a schedule for the websites that are hardest to resist — social media, gaming, streaming — and enforces it without giving you (or anyone else) an easy way out. It runs silently in the background as a system service. Turning it off requires your admin password.

It works across every browser and every app, not just the one you happen to have open. There's no extension to disable, no toggle to flick.

## Frequently asked questions

### Why not a browser extension?

Browser extensions are easy to bypass — one click to disable, and kids know it. Sentinel works at the operating system level: it rewrites the system hosts file (or runs a local DNS resolver in advanced mode), so blocked sites don't load anywhere on the machine, in any browser or app. Getting around it requires admin credentials.

### How does this compare to DNS blockers like AdGuard Home?

AdGuard Home is a solid network-wide DNS blocker, and Sentinel can run alongside it (see [Running alongside AdGuard Home](TROUBLESHOOTING.md#running-sentinel-alongside-adguard-home-or-any-other-local-dns-service)). But its scheduling model has hard limits that make fine-grained personal productivity rules impractical — all three are tracked as open feature requests on their repo:

| Capability | Sentinel | AdGuard Home |
|---|---|---|
| Multiple block windows per day per group | ✅ e.g. 9–12 and 2–6 | ❌ One slot per day ([#7253](https://github.com/AdguardTeam/AdGuardHome/issues/7253)) |
| Independent schedule per domain group | ✅ Each rule is separate | ❌ One shared schedule for all ([#7146](https://github.com/AdguardTeam/AdGuardHome/issues/7146)) |
| Custom domain groups | ✅ Any domains you choose | ❌ Predefined catalog only ([#1692](https://github.com/AdguardTeam/AdGuardHome/issues/1692)) |
| Survives browser DNS-over-HTTPS (Secure DNS) | ✅ `hosts` & `strict` modes | ❌ DoH skips it |
| Kernel-level IP blocking (pf firewall) | ✅ `strict` mode (macOS) | ❌ DNS-only |
| Browser tab auto-close on block | ✅ macOS | ❌ |
| Pre-block notifications | ✅ macOS | ❌ |

AdGuard Home excels at network-wide content filtering — blocking ad trackers or adult content for every device on a home network. Sentinel is built for personal schedule enforcement on a single machine: granular enough to say "block Reddit 9–12 and 2–6, block gaming all evening, and leave streaming open on weekends." It also stays effective against a browser configured for DNS-over-HTTPS, which AdGuard Home cannot — see [the DoH FAQ](#what-about-browsers-using-dns-over-https-doh-or-secure-dns) below for how each Sentinel mode handles that. If you already run AdGuard Home, see [Running alongside AdGuard Home](TROUBLESHOOTING.md#running-sentinel-alongside-adguard-home-or-any-other-local-dns-service) to run both together.

### What about browsers using DNS-over-HTTPS (DoH or "Secure DNS")?

Sentinel handles all three common DoH cases:

1. **Default browser installs.** Chrome's automatic Secure DNS only upgrades to DoH when the system DNS is a *known* provider (8.8.8.8, 1.1.1.1, etc.). Sentinel sets system DNS to `127.0.0.1` (the local proxy), which is not on Chrome's list, so automatic mode stays on regular DNS and goes through the proxy normally. **No action needed.**
2. **`hosts` mode.** `getaddrinfo` reads `/etc/hosts` before any resolver, so blocked entries take effect even when the browser is using DoH. **No action needed.**
3. **`strict` mode (recommended for advanced users).** Even if a user has manually set "With Google" / "With Cloudflare" in `chrome://settings/security`, strict mode still blocks because (a) it resolves the real IPs of blocked domains and drops outbound packets at the kernel via `pf`, and (b) it bundles an always-on `_doh` group of common DoH/DoT endpoints (`dns.google`, `cloudflare-dns.com`, `mozilla.cloudflare-dns.com`, `dns.quad9.net`, …) and blocks them on TCP/443 (DoH) and TCP+UDP/853 (DoT) so the browser can't reach the DoH provider in the first place. UDP/53 plain DNS to those CDNs is left open so the daemon's own `backup_dns` keeps working. **No action needed.**

The only mode DoH can bypass is `dns`-only mode combined with a browser that has a *manually-configured* DoH provider — switch that machine to `hosts` or `strict` and you're covered. See [TROUBLESHOOTING.md § Browser DNS-over-HTTPS bypass](TROUBLESHOOTING.md#browser-dns-over-https-doh-bypass) for verification commands and a deeper walkthrough.

### Does Sentinel keep my laptop awake?

No. The scheduler's 1-minute ticker is a regular Go timer — it doesn't issue power assertions and isn't visible to macOS as user activity. The launchd plist sets no `ProcessType: Interactive` or `KeepAlive`, and the DNS and web servers bind to `127.0.0.1` only, so they never initiate outbound network activity. Your Mac sleeps on its normal idle schedule; when it wakes, the ticker resumes and the next rule evaluation happens within ~60 s. To confirm: run `pmset -g assertions` while the daemon is active — Sentinel will not appear in the assertions list.

---

<!-- Screenshot: dashboard Status tab — coming soon -->

---

## Table of contents

- [FAQ](#frequently-asked-questions)
- [What it does](#what-it-does)
- [Platform support](#platform-support)
- [Install](#install)
- [The web dashboard](#the-web-dashboard)
- [Configuration](#configuration)
- [Profiles](#profiles)
- [Pause & resume](#pause--resume)
- [Focus sessions (Pomodoro)](#focus-sessions-pomodoro)
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
- **Per-group granularity, multiple windows per day** — each domain group has its own independent schedule. Stack multiple block/allow windows in the same day (e.g. block social media 9–12 and 2–6, block gaming from 8pm onward — all as separate rules).
- **Daily quotas (allowance mode)** — optionally cap how long a group is accessible each day. Add `"daily_quota_minutes": 30` to any rule and the group is blocked for the rest of the day once 30 minutes of active DNS usage is consumed — even if the scheduled window is still open. Requires `dns` or `strict` mode.
- **Foreground time tracking (opt-in)** — flip `"enable_foreground_tracking": true` and the dashboard shows how long each configured site was the *active* browser tab, idle-aware and ignoring background tabs. Browser-only, works under any enforcement mode — including `hosts`, where DNS-derived usage isn't available. Privacy-scoped to domains you've already configured for blocking. Full fidelity on macOS (Chrome, Safari, Arc, Brave). On Windows it defaults to a coarse window-title heuristic for Chrome and Edge (only catches pages whose title contains the domain); add `"windows_foreground_use_uia": true` to also read the real address bar via UI Automation.
- **Tabs close automatically** — every minute during an active block, Chrome, Safari, Arc, and Brave have any open tabs for blocked sites closed. This catches not just the moment the block begins but also tabs opened *during* the window (e.g., via Safari iCloud Private Relay or browser DoH that bypass DNS-layer blocking). No willpower required. (macOS)
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
| Foreground tab time tracking | ✅ Chrome, Safari, Arc, Brave | ⚠ Chrome, Edge — window-title heuristic; address-bar via UI Automation opt-in |
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

Open **`http://localhost:8040`** while the service is running. The header displays the running version (e.g. `v0.1.19`) — handy for confirming whether a release update has actually rolled over to the live daemon.

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

Configuration lives in two places on disk: a **bootstrap file** that holds settings, the shared groups dictionary, pause/pomodoro state, and the active-profile pointer; and one or more **profile files** that hold only the rules. This split lets you swap between named rule sets (Work, Study, Weekend, …) without re-saving everything else. See [Profiles](#profiles) for the full feature.

| OS | Bootstrap | Profiles |
|---|---|---|
| macOS (service)   | `/Library/Application Support/Sentinel/sentinel.json` | `/Library/Application Support/Sentinel/profiles/<name>.json` |
| Windows (service) | `%PROGRAMDATA%\Sentinel\sentinel.json`                | `%PROGRAMDATA%\Sentinel\profiles\<name>.json`                |
| Any (`--local`)   | `./sentinel.json`                                     | `./profiles/<name>.json`                                     |

Both files are created with defaults on first launch. The scheduler reloads from disk every minute — live edits take effect on the next tick, no restart required. **Migration**: if you're upgrading from a pre-profiles install, your existing `config.json` is split into the bootstrap + a `profiles/default.json` automatically on first run, and the original is renamed to `config.json.bak` as a safety net.

### Example config

The bootstrap (`sentinel.json`) holds everything except rules:

```json
{
  "settings": {
    "primary_dns": "8.8.8.8:53",
    "backup_dns": "1.1.1.1:53",
    "enforcement_mode": "hosts",
    "dns_failure_mode": "open",
    "enable_foreground_tracking": false,
    "windows_foreground_use_uia": false,
    "auth_token": "<auto-generated on first launch>"
  },
  "groups": {
    "games":  ["roblox.com", "rbxcdn.com", "epicgames.com", "steampowered.com", "fortnite.com", "minecraft.net", "..."],
    "videos": ["youtube.com", "googlevideo.com", "ytimg.com", "twitch.tv", "ttvnw.net", "netflix.com", "nflxvideo.net", "..."],
    "social": ["facebook.com", "fbcdn.net", "facebook.net", "instagram.com", "cdninstagram.com", "tiktok.com", "tiktokv.com", "tiktokcdn.com", "..."],
    "_doh":   ["dns.google", "cloudflare-dns.com", "mozilla.cloudflare-dns.com", "one.one.one.one", "..."]
  },
  "active_profile": "default"
}
```

A profile file (`profiles/default.json`) holds only the rules:

```json
{
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

The default config ships with three active groups plus one opt-in. `games` and `videos` are blocked during focus windows — mornings (9–12), afternoons (2–4), and an evening hour (6–7) on weekdays; mornings and the evening hour on weekends. `social` is blocked all day, every day. Each group includes the platform's CDN/asset domains (`fbcdn.net`, `googlevideo.com`, `tiktokv.com`, etc.) — without these, browsers can still load most page content even when the apex domain is blocked. See `internal/config/default_config.json` in the repo for the complete lists.

The `_doh` group is **active by default in strict mode**. It contains common DNS-over-HTTPS endpoints (`dns.google`, `cloudflare-dns.com`, `mozilla.cloudflare-dns.com`, etc.) and exists to defeat browsers that bypass system DNS. In `strict` mode it gets a hybrid treatment: blocked at the DNS layer (so apps that look up these hostnames get `0.0.0.0`) **and** in pf with port-restricted rules — TCP/443 (DoH) and TCP+UDP/853 (DoT) are dropped on the resolved IPs while UDP/53 plain DNS stays reachable, so the daemon's own `backup_dns` (typically `1.1.1.1:53`) keeps working. The pf anchor file you'll see at `/etc/pf.anchors/sentinel` has two clearly-labeled sections reflecting this. Set `is_active: false` on the `_doh` rule if it interferes with your setup.

### Field reference

- **`settings.enforcement_mode`** — `"hosts"` (default), `"dns"`, or `"strict"`. See [Enforcement modes](#enforcement-modes).
- **`settings.primary_dns` / `backup_dns`** — upstream resolvers used in `dns`/`strict` mode. Ignored in `hosts` mode.
- **`settings.dns_failure_mode`** — `"open"` (default) or `"closed"`. Controls what happens if Sentinel stops unexpectedly while in `dns`/`strict` mode. `"open"`: the OS DNS is set to `127.0.0.1 <backup_dns_host>`, so if Sentinel crashes the machine falls through to `backup_dns` and stays online (blocking lapses until launchd restarts the service). `"closed"`: only `127.0.0.1` is set — DNS fails entirely while Sentinel is down, making a crash unbypassable. Requires `backup_dns` to be a non-loopback IP on port 53; if it isn't, `"open"` silently behaves like `"closed"` and logs a warning. Ignored in `hosts` mode.
- **`settings.auth_token`** — auto-generated on first launch. The web UI sends this in `X-Auth-Token` for every mutating API call.
- **`settings.enable_foreground_tracking`** *(optional, default `false`)* — opt-in flag for foreground-tab time tracking. When on, the scheduler probes every minute to record how long the active browser tab spends on a configured (non-`_doh`) domain. Idle-aware (≥60s of no input → don't count). Records nothing for tabs whose host isn't already in one of your groups, so you cannot accidentally start logging unrelated browsing. Aggregated separately from `used_minutes` and surfaced as `foreground_minutes` on `/api/usage`. Works under any enforcement mode, including `hosts` (where DNS-derived usage isn't available). Does **not** affect `daily_quota_minutes` — quotas still drive off the DNS-bucket signal. **macOS** reads the real active-tab URL from Chrome/Safari/Arc/Brave via AppleScript. **Windows** reads the foreground window's title (Chrome and Edge only) and extracts a host from it, so by default it only registers pages whose title contains the domain — see `windows_foreground_use_uia` to do better. On Linux/other it's a no-op.
- **`settings.windows_foreground_use_uia`** *(optional, default `false`, Windows-only)* — when on, the Windows foreground probe additionally reads the active tab's real URL out of the browser's address bar via UI Automation, falling back to the window-title heuristic if that fails (or if the browser UI isn't English — only a handful of locales are recognised so far). Requires `enable_foreground_tracking`. It's the accurate mode but exercises a fair amount of COM plumbing, so it's opt-in for now; see issue #94.
- **`groups`** — named lists of domains that rules are bound to. In `hosts` mode, common prefixes (`www.`, `m.`, `mobile.`, `app.`) are blocked automatically. In `dns` mode, subdomain matching is suffix-based (`a.b.example.com` is blocked if `example.com` is in the group).
- **`rules[].group`** — must match a key in `groups`.
- **`rules[].is_active`** — set to `false` to suspend a rule without deleting it.
- **`rules[].daily_quota_minutes`** *(optional)* — cap how many minutes per calendar day the group is accessible. Once this limit is reached the group is blocked for the rest of the day, regardless of the scheduled window. `0` or omitted means no quota. **Requires `dns` or `strict` enforcement mode** — usage is tracked at the DNS proxy layer and is not available in `hosts` mode. Browsers in their default configuration are unaffected. The edge case: if a browser has a **specific DoH provider manually configured** (Chrome set to "With Google" / "With Cloudflare"; Firefox "DNS over HTTPS" set to a provider), it bypasses `127.0.0.1:53` and those queries are not counted. Leaving browser DNS on automatic (the default) avoids this.
- **`rules[].schedules`** — keyed by weekday (`"Monday"` … `"Sunday"`). Each value is an array of `{start, end}` slots in `HH:MM` 24-hour format. Domains are blocked when the current time falls in `[start, end)`.
- **`pause`** *(optional)* — `{"until": "<RFC3339 timestamp>"}`. All blocking suspended until that time. Cleared automatically when the timestamp passes.

---

## Profiles

A **profile** is a named set of rules. Sentinel ships with one (`default`); you can save more (`work`, `study`, `weekend`, …) and switch between them. Settings, the shared groups dictionary, and runtime state (pause/pomodoro) live in the bootstrap and are *not* per-profile, so a switch only swaps the schedule — it doesn't reset your DNS config, your auth token, or break a Pomodoro work-phase lock.

### How files are laid out

```
$CONFIG_DIR/
├── sentinel.json              # bootstrap: settings, groups, pause, pomodoro,
│                              # auth_token, "active_profile": "default"
└── profiles/
    ├── default.json           # rules only
    ├── work.json              # rules only
    └── study.json             # rules only
```

Each profile file is just `{"rules": [ … ]}` — same `Rule` shape as before. Group references (`"group": "social"`) resolve against the shared groups dictionary in the bootstrap, so to add profile-specific blocking you generally define a finer-grained group (e.g., `youtube_only` separate from `videos`) rather than overriding inside the profile.

### Switching profiles

From the dashboard, the header has a profile dropdown that's visible on every tab. Picking a different profile POSTs to `/api/profile/switch` and the daemon picks up the new active profile on its next 1-minute scheduler tick. The dropdown is auto-disabled during a Pomodoro work session (HTTP 423) so you can't unlock yourself out of focus mode by switching to an empty profile.

From the CLI:

```bash
# List profiles, '*' marks the active one
sudo sentinel --list-profiles

# Switch to a different profile (root needed because this writes the system bootstrap)
sudo sentinel --set-profile work
```

### Creating, cloning, and deleting profiles

The **Manage** tab has a "Profiles" section (gated by the same PIN that protects rule edits). Type a name, optionally pick a profile to clone from, hit Create. To delete a profile, switch away from it first — the active profile cannot be deleted, and `default` is permanently undeletable.

Or via the API directly:

```bash
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')

# Create a "work" profile cloned from default
curl -X POST http://localhost:8040/api/profiles \
  -H "X-Auth-Token: $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"work","clone_from":"default"}'

# Switch active profile
curl -X POST http://localhost:8040/api/profile/switch \
  -H "X-Auth-Token: $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"work"}'

# Delete an inactive profile
curl -X DELETE http://localhost:8040/api/profiles/work \
  -H "X-Auth-Token: $TOKEN"
```

Profile names must match `^[a-z0-9][a-z0-9_-]{0,31}$` (lowercase, dashes/underscores, max 32 chars). The names `sentinel`, `bootstrap`, and `config` are reserved.

### Migration from older installs

If you're upgrading from a version that used a single `config.json`, Sentinel migrates automatically the first time the new daemon boots: `Settings`, `Groups`, `Pause`, and `Pomodoro` go into `sentinel.json`; `Rules` go into `profiles/default.json`; the old file is renamed to `config.json.bak` so a botched migration is recoverable. The migration is idempotent — re-running it does nothing once the new layout is in place.

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

## Focus sessions (Pomodoro)

A focus session is the inverse of pause: a fixed work interval where blocking is *forced on* regardless of your normal schedule, followed by a break interval where it's *released*. Open the dashboard's **Status** tab, set the work and break durations (defaults: 25 min / 5 min; allowed ranges 1–120 work, 1–60 break), and click **Start Focus Session**.

**During the work phase** every active rule is treated as blocking right now, even rules whose schedule wouldn't fire at this time. The pause endpoint and the config-update endpoint both return `423 Locked` — you can't pause out, edit your way out, or stop the session early. When the work phase ends, a macOS notification fires and the daemon transitions automatically into the break phase.

**During the break phase** all rules revert to their normal schedule (so most things are unblocked, unless your usual schedule blocks them anyway). A second notification fires when the break ends, and the session clears itself; sessions don't auto-restart.

Via the API:

```bash
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')

# Start a 50-minute work / 10-minute break session
curl -X POST http://localhost:8040/api/pomodoro/start \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"work_minutes": 50, "break_minutes": 10}'

# Stop a session (only allowed during the break phase)
curl -X DELETE http://localhost:8040/api/pomodoro \
  -H "X-Auth-Token: $TOKEN"
```

Session state is persisted in `config.json` under the optional `pomodoro` field (`{phase, phase_ends_at, work_minutes, break_minutes}`). It survives daemon restart and laptop sleep — when the system wakes, the next scheduler tick (within ~60 s) re-evaluates the phase and transitions or clears as needed.

> **Note:** the work-phase lock is an enforcement convenience, not a security boundary — anyone with admin access to your machine can still edit `config.json` directly to drop the session. The point is to add a couple of layers of friction during focused work.

---

## Enforcement modes

Three modes are available. **`strict` is recommended on macOS** — it combines DNS-proxy blocking with kernel-level firewall rules so no app can route around it.

### `strict` ⭐ recommended on macOS

Runs a local DNS proxy on `127.0.0.1:53` **and** installs a `pf` (Packet Filter) anchor that drops outbound packets to the resolved IPs at the kernel level. On every scheduler tick the enforcer re-resolves all blocked domains and rewrites the firewall table from scratch, so CDN IP rotation doesn't create gaps in coverage.

**Best for:** macOS users who want the strongest blocking. Works against apps that hard-code their own DNS resolver (some VPNs, browsers with a specific DoH provider manually configured) because the firewall blocks the IPs directly, not just the names. **Note:** pf blocks connections but does not intercept DoH traffic, so quota tracking can be inaccurate if a browser has a specific DoH provider manually set — leaving browser DNS on automatic (the default) avoids this.

**macOS only.** If pf setup fails (e.g. missing root), the enforcer degrades to DNS-only and logs a warning.

**Requires:** pointing your OS DNS at `127.0.0.1` (see [Install](#install) advanced note).

### `dns`

Runs a local DNS proxy on `127.0.0.1:53`. Blocked domains return `0.0.0.0` (A) and `::` (AAAA); everything else forwards to `primary_dns` (failover to `backup_dns`).

**Best for:** users who need wildcard subdomain blocking — any `*.example.com` subdomain is blocked if `example.com` is in the group — but don't need firewall-layer enforcement.

**Requires:** pointing your OS DNS at `127.0.0.1` (see [Install](#install) advanced note). Apps that hard-code their own resolver bypass it. Browsers in their default configuration are unaffected — Chrome's automatic mode will not upgrade to DoH when system DNS is `127.0.0.1`. The exception is browsers with a **specific DoH provider manually configured**, which bypass the proxy regardless of system DNS.

### `hosts` (default, cross-platform)

Edits `/etc/hosts` (macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows). Blocked entries live between marker lines (`# sentinel:begin` / `# sentinel:end`) and are atomically rewritten — a crash mid-write cannot corrupt the file. Other entries are never touched.

**Best for:** Windows users, or macOS users who prefer the simplest setup with no port binding or DNS reconfiguration.

**Limitation:** no wildcards — only a static prefix list (`www.`, `m.`, `mobile.`, `app.`), so very deep CDN subdomains aren't covered.

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
| `--local` | none | Run the daemon in the foreground using `./sentinel.json` + `./profiles/` | [Test utilities](#test-utilities) |
| `--set-mode <mode>` | sudo | Set `enforcement_mode` in config and exit (modes: `hosts`, `dns`, `strict`) | [Switching modes](#switching-modes) |
| `--set-profile <name>` | sudo | Switch the active profile and exit; the daemon picks it up within 60s | [Profiles](#profiles) |
| `--list-profiles` | sudo | Print every saved profile, marking the active one with `*` | [Profiles](#profiles) |
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
| `/api/usage` | GET | Per-group and per-domain DNS usage minutes; `?range=today\|7d\|30d` |

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

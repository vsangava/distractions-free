# Sentinel — System Design

This document describes how the daemon is built. It complements [README.md](./README.md), which is the user-facing guide, and [TROUBLESHOOTING.md](./TROUBLESHOOTING.md), which covers diagnostics and recovery.

## Contents

1. [Overview](#1-overview)
2. [System architecture](#2-system-architecture)
3. [Module layout](#3-module-layout)
4. [The enforcer abstraction](#4-the-enforcer-abstraction)
5. [Scheduler](#5-scheduler)
6. [DNS proxy](#6-dns-proxy)
7. [pf (Packet Filter) integration](#7-pf-packet-filter-integration)
8. [Configuration](#8-configuration)
9. [Web server & HTTP API](#9-web-server--http-api)
10. [Process lifecycle](#10-process-lifecycle)
11. [Cleanup (`clean`)](#11-cleanup-clean)
12. [Testability strategy](#12-testability-strategy)
13. [Concurrency model](#13-concurrency-model)
14. [Security model](#14-security-model)
15. [Platform-specific notes](#15-platform-specific-notes)

---

## 1. Overview

Sentinel is a single-binary Go daemon that runs as a privileged system service. It enforces per-time-of-day blocking rules on the host — each rule binds a named group of domains (e.g. `games`, `social`) to a weekly schedule — so distracting sites become unreachable during configured windows.

The interesting design decisions:

- **Three interchangeable enforcement backends.** Blocking can be done by editing `/etc/hosts` (default), by running a local DNS proxy on `127.0.0.1:53`, or by combining the DNS proxy with a `pf` packet-filter anchor on macOS. The scheduler does not know which is in use; it talks to an `Enforcer` interface.
- **Pure functions for the parts worth testing.** Rule evaluation, DNS-response building, hosts-line generation, and pf-anchor generation are pure functions of `(time, config, …)` with no side effects. This means the test suite covers the core behaviour without needing root, port 53, or system-file access.
- **Diff-based activation.** The scheduler runs once a minute and computes the diff against the previous tick. The enforcer is given only `newlyBlocked` and `newlyUnblocked` lists, not the full set, so a steady-state minute is a no-op.
- **Auto-reloading config.** Every tick reloads `config.json` from disk. There is no inotify/FSEvents listener — the 60-second window is the maximum staleness, and is good enough for human-edited rules.
- **Dashboard embedded in the binary.** Static HTML/CSS/JS is bundled via `go:embed`, so a single binary is the entire deliverable.

### Tech stack

- **Language:** Go (cross-compiles to `darwin/arm64`, `darwin/amd64`, `windows/amd64`).
- **DNS:** [`github.com/miekg/dns`](https://github.com/miekg/dns).
- **Service framework:** [`github.com/kardianos/service`](https://github.com/kardianos/service) — abstracts launchd / Windows Service Manager.
- **Web:** `net/http` only.
- **External binaries called:** `osascript`, `networksetup`, `dscacheutil`, `killall`, `pfctl` (macOS); `ipconfig`, `powershell` (Windows).

---

## 2. System architecture

```
                                 ┌─────────────────────────────┐
                                 │  config.json (auto-reloaded │
                                 │  every minute from disk)    │
                                 └──────────────┬──────────────┘
                                                │
                                                ▼
   ┌─────────────────────────────────────────────────────────────────┐
   │                       Scheduler (1-min tick)                     │
   │  • Loads config                                                  │
   │  • Calls EvaluateRulesAtTime(now, cfg) → set of blocked domains  │
   │  • Diffs against previous tick → newlyBlocked, newlyUnblocked    │
   │  • Calls enforcer.Activate / .Deactivate with the diffs          │
   │  • Calls CheckWarningDomainsAtTime — fires AppleScript notification
   │  • On block start, fires AppleScript to close Chrome/Safari tabs │
   └──────────────────────────┬──────────────────────────────────────┘
                              │
                              ▼
            ┌───────────────────────────────────────┐
            │    enforcer.Enforcer (interface)      │
            │       Setup / Teardown                │
            │       Activate / Deactivate / All     │
            └───┬───────────────┬──────────────┬────┘
                │               │              │
        ┌───────▼─────┐  ┌──────▼──────┐  ┌────▼─────────┐
        │ HostsEnforce│  │ DNSEnforcer │  │ StrictEnforce│
        │  edits      │  │  updates    │  │  composes    │
        │  /etc/hosts │  │  blocked    │  │  DNS + pf    │
        │             │  │  map →      │  │              │
        │             │  │  proxy pkg  │  │              │
        └─────────────┘  └──────┬──────┘  └──┬───────┬───┘
                                │            │       │
                                ▼            ▼       ▼
                       ┌──────────────┐  ┌─────────────┐
                       │  proxy.DNS   │  │  pf anchor  │
                       │  Server      │  │  /etc/pf.   │
                       │  127.0.0.1:53│  │  anchors/   │
                       └──────────────┘  └─────────────┘

   ┌─────────────────────────────────────────────────────────────────┐
   │              Web server on 127.0.0.1:8040                       │
   │  • GET  /api/config            (public — bootstraps auth token) │
   │  • POST /api/config/update     (auth)                           │
   │  • GET/POST /api/status        (auth)                           │
   │  • GET/POST /api/test-query    (auth)                           │
   │  • GET/POST /api/hosts-preview (auth)                           │
   │  • GET/POST /api/pf-preview    (auth)                           │
   │  • POST /api/pause             (auth)                           │
   │  • DELETE /api/pause           (auth)                           │
   │  • Embedded static dashboard (Status / Test / Manage tabs)      │
   └─────────────────────────────────────────────────────────────────┘
```

### Request paths

**`hosts` mode (default):** the OS resolves names normally; `/etc/hosts` short-circuits the lookup for blocked domains. No port 53 is bound. The scheduler is the only thing the daemon needs to do.

**`dns` mode:** the OS is configured to use `127.0.0.1:53` as its resolver. Each query is checked against the in-memory `blocked` map. Blocked A-record queries return `0.0.0.0`; everything else is forwarded to the upstream resolver (with backup-DNS failover).

**`strict` mode:** like `dns`, plus the scheduler asks the pf enforcer to resolve each newly-blocked domain to A/AAAA addresses and load them into a `<blocked_ips>` table inside the `sentinel` pf anchor. Outbound packets to those IPs are dropped at the kernel.

---

## 3. Module layout

```
cmd/app/main.go                  # entry point: dispatch and service wiring
internal/
  config/config.go               # JSON config + thread-safe in-memory copy
  enforcer/
    enforcer.go                  # Enforcer interface + factory + flushDNSCache
    hosts.go                     # HostsEnforcer — edits /etc/hosts
    dns.go                       # DNSEnforcer  — updates proxy's blocked map
    strict.go                    # StrictEnforcer — DNS + pf composition
  scheduler/scheduler.go         # 1-min ticker, diff computation, AppleScript
  proxy/dns.go                   # DNS server, GetDNSResponse pure function
  pf/pf.go                       # pf anchor management (macOS, root)
  cleanup/cleanup.go             # clean implementation: per-step, idempotent
  cleanup/priv_unix.go           # IsPrivileged() — Geteuid()==0
  cleanup/priv_windows.go        # IsPrivileged() — token IsMember Administrators
  testcli/testcli.go             # --test-query CLI + GetQueryResult struct
  web/server.go                  # HTTP handlers, auth middleware
  web/static/index.html          # embedded SPA (Status/Test/Manage tabs)
```

Dependency direction:

```
cmd/app/main.go
  ↓
  ├── config        ← used by everything
  ├── enforcer ──→ proxy, pf, config
  ├── scheduler ──→ enforcer, config
  ├── proxy ──→ config
  ├── web ──→ scheduler, enforcer (preview), pf (preview), testcli, config
  ├── cleanup ──→ enforcer (hosts cleanup), pf, config
  └── testcli ──→ scheduler, proxy, config
```

There are no circular imports. The `proxy` package has no dependency on `enforcer` or `scheduler` — the data flow is one-way: scheduler → enforcer → proxy.

---

## 4. The enforcer abstraction

The core extension point. Defined in `internal/enforcer/enforcer.go`:

```go
type Enforcer interface {
    Setup() error                       // called once at service start
    Teardown() error                    // called once at service stop
    Activate(domains []string) error    // newly-blocked diff
    Deactivate(domains []string) error  // newly-unblocked diff
    DeactivateAll() error               // remove every block (used on shutdown)
}

func New(cfg config.Config) Enforcer {
    switch cfg.Settings.GetEnforcementMode() {
    case "strict": return NewStrictEnforcer(cfg)
    case "dns":    return NewDNSEnforcer(cfg)
    default:       return NewHostsEnforcer(cfg)   // includes empty / unrecognised
    }
}
```

`Activate` and `Deactivate` are called with **diffs**, not the full blocked set. The scheduler is responsible for computing the diff from one tick to the next. This keeps the enforcer implementations simple and means a steady-state minute is a no-op.

### `HostsEnforcer` (`hosts.go`)

Edits `/etc/hosts` (or `C:\Windows\System32\drivers\etc\hosts`) between marker lines:

```
# sentinel:begin
0.0.0.0 youtube.com
0.0.0.0 www.youtube.com
0.0.0.0 m.youtube.com
0.0.0.0 mobile.youtube.com
0.0.0.0 app.youtube.com
# sentinel:end
```

Subdomain prefixes are hardcoded: `["", "www.", "m.", "mobile.", "app."]`. There is no wildcard support in `/etc/hosts`, so this is a deliberate, conservative list. Adding more would inflate the hosts file for marginal benefit.

Writes are atomic: the new contents are written to `/etc/hosts.df.tmp` then renamed over `/etc/hosts`. A crash mid-write cannot leave the file half-rewritten.

Activate is idempotent — entries already present are not duplicated. DeactivateAll removes the entire managed block (markers and all) so the file is restored to its pre-installation form.

After every edit the enforcer flushes the OS resolver cache via `flushDNSCache()` so changes take effect without waiting for TTL.

The `GenerateHostsEntries(domains []string) []string` function is exported so the web UI's `/api/hosts-preview` endpoint can show what *would* be written without root.

### `DNSEnforcer` (`dns.go`)

Maintains an in-memory `blocked map[string]bool` and pushes updates to the `proxy` package via `proxy.UpdateBlockedDomains(snapshot)` on every Activate/Deactivate. The DNS server itself is started by `cmd/app/main.go`, not by the enforcer — this separation matters because the server is the thing that blocks the goroutine forever, and `main.go` needs to control that.

`Teardown` resets the system DNS to its previous state. **Caveat:** the current implementation only resets the `Wi-Fi` interface on macOS (and `Wi-Fi` on Windows). Multi-interface cleanup is delegated to the `clean` command (see `internal/cleanup/cleanup.go`), which iterates every interface that points at `127.0.0.1`. Tracked in issue #12.

### `StrictEnforcer` (`strict.go`)

Composes a `DNSEnforcer` with the `pf` package. On `Setup` it tries to install the pf anchor; if that fails (non-darwin, missing `pfctl`, edited `pf.conf`) it logs a warning and continues with DNS-only enforcement — a graceful degradation rather than a hard failure.

On `Activate(domains)` it calls `dns.Activate(domains)` then `pf.ActivateBlock(domains, primaryDNS)`, which resolves the domains to IPs and reloads the pf table.

On `Deactivate(domains)` it calls `dns.Deactivate(domains)` then **rebuilds the pf table from scratch** using whatever's still in the DNS-blocked set. Selective IP removal isn't worth the complexity given how few domains are typically active simultaneously.

---

## 5. Scheduler

`internal/scheduler/scheduler.go`. The orchestrator. Two responsibilities:

1. Drive a 1-minute ticker that re-evaluates rules and tells the enforcer about changes.
2. Fire the macOS-specific UI side effects: 3-minute warning notification and per-tick tab-closing AppleScript.

### The pure functions

```go
func EvaluateRulesAtTime(t time.Time, cfg config.Config, quotaUsage map[string]int) map[string]bool
func CheckWarningDomainsAtTime(t time.Time, cfg config.Config) []string
```

`EvaluateRulesAtTime` returns the set of domains that should be blocked at `t`. Algorithm:

1. If `cfg.IsPaused(t)` — return empty map (pause overrides everything).
2. If `cfg.IsLockedByPomodoro(t)` — return the union of every active rule's resolved domains (focus session forces blocking on, ignoring schedules and quota). See section 8 for details.
3. For each rule with `is_active=true`, resolve `cfg.ResolveGroup(rule.Group)` to a domain list (skip the rule if the group is missing or empty), then look up `Schedules[t.Weekday().String()]`.
4. For each slot, parse `Start`/`End` as `15:04`, build a same-day comparison time, check `slotStart <= now < slotEnd`.
5. On the first matching slot, add every domain in the resolved group to the result and stop checking remaining slots for that rule.
6. After schedule evaluation, if `quotaUsage` is non-nil, add every domain from any rule whose group has `used >= DailyQuotaMinutes` — quota-exhausted groups are blocked for the rest of the day regardless of the schedule window.

`quotaUsage` maps group name → minutes used today (computed from DNS usage buckets). Callers pass `nil` when no quota enforcement is needed (e.g. test utilities, `--test-web` mode).

`CheckWarningDomainsAtTime` returns domains whose block starts within `[now, now+3min)`. Logic mirrors the above but compares against the slot's start time, building a warning window of `[start-3min, start)`.

Both functions are deterministic given their inputs. Tests construct synthetic `time.Time` and `config.Config` values and pass `nil` for `quotaUsage`.

### The tick loop

```go
func Start() {
    ticker := time.NewTicker(1 * time.Minute)
    go func() {
        evaluateRules()              // run once immediately
        for range ticker.C {
            evaluateRules()
        }
    }()
}

func evaluateRules() {
    config.LoadConfig()
    cfg := config.GetConfig()
    now := time.Now()

    // 1. Self-clear an expired pause window so config.json stays clean.
    if cfg.Pause != nil && !now.Before(cfg.Pause.Until) {
        config.ClearPause()
        config.SaveConfig()
        cfg = config.GetConfig()
    }

    newBlocked := EvaluateRulesAtTime(now, cfg)
    warningDomains := CheckWarningDomainsAtTime(now, cfg)

    // 2. Diff against the previous tick.
    var newlyBlocked, newlyUnblocked []string
    // ... (set difference)

    // 3. Push diff through the active enforcer.
    if len(newlyBlocked) > 0 {
        activeEnforcer.Activate(newlyBlocked)
    }
    if len(newlyUnblocked) > 0 {
        activeEnforcer.Deactivate(newlyUnblocked)
    }

    // 4. Per-tick tab closer (every tick, not just on transitions). Catches tabs
    //    that survived the initial block — opened mid-window via DoH bypass,
    //    iCloud Private Relay, memorized IPs, or stale reloads. Filters _doh
    //    group out of the probe set before checking browsers, since DoH
    //    endpoints aren't sites users visit with browsers.
    runPerTickCloseTabs(newBlocked, cfg, browserTabProbe)

    // 5. Fire warnings, debounced per-domain to once per minute.
    if len(warningDomains) > 0 {
        // for each domain, check lastWarningTime[domain]; if >=1min ago, run.
        runMacOSWarning(domainsToWarn)
    }
}
```

The `activeEnforcer` is wired by `main.go` via `scheduler.SetEnforcer(e)` before `Start()` is called.

`closeMacOSTabs` and `runMacOSWarning` are no-ops on non-darwin. On darwin they call into the AppleScript abstraction (next section).

`runPerTickCloseTabs` first applies the `_doh` filter to the blocked set, then probes browsers via `browserTabProbe` (a swappable `func([]string) []string`). Only when the probe returns at least one open match does it fire `closeMacOSTabs`, so the steady-state cost when no browsers are open is one cheap osascript probe per tick. The reason for running every tick rather than only on transitions: in strict mode, IP-layer enforcement has fundamental gaps (Safari iCloud Private Relay, browser DoH, geo-anycast IP mismatches), so a tab opened *during* an active block window — when there's no transition for the close path to hook on — would otherwise stay open forever. See issue #81.

### Daily quota tracking

Each scheduler tick also:

1. Calls `BuildGroupLookup(cfg)` — builds a `map[string]string` of domain → group for *all* configured domains (not just currently-blocked ones) and pushes it to `proxy.UpdateGroupLookup`. The proxy uses this to log non-blocked queries.
2. Reads `usage.jsonl` for today's events and computes minutes-used per quota group via `proxy.ComputeAllGroupUsageMinutes`.
3. Passes the resulting `quotaUsage` map to `EvaluateRulesAtTime`, which blocks any group whose usage ≥ `DailyQuotaMinutes`.

**Usage measurement:** `proxy/usagelog.go` records each non-blocked DNS query for a group domain as a `UsageEvent{TS, Domain, Group}` in `{configDir}/usage.jsonl`. Usage is aggregated in 5-minute buckets (`TS.Unix() / 300`) and usage minutes = `distinct buckets × 5`. The 5-minute window deduplicates Chrome's aggressive DNS re-resolution (TTL capped at 60s internally) while keeping granularity fine enough for quotas of 15+ minutes.

**Known limitations:**
- Tracking requires `dns` or `strict` mode. In `hosts` mode the proxy never sees queries — see *Foreground tab tracking* below for the complementary signal that does work in `hosts` mode.
- **Browsers with a specific DoH provider manually configured bypass usage tracking.** Chrome's automatic mode will use the system resolver (`127.0.0.1`) and is unaffected — it only upgrades to DoH when the system DNS is a known provider (8.8.8.8, 1.1.1.1), and Sentinel replaces that with `127.0.0.1`. However, if a user has explicitly set a specific DoH provider in browser settings (Chrome: "With Google" / "With Cloudflare"; Firefox: "DNS over HTTPS" set to a provider), queries are sent directly over HTTPS on port 443, completely skipping `127.0.0.1:53`. Those queries never reach the proxy, so no usage event is recorded and quota never fills up. `strict` mode does not fix this — pf blocks connections but does not intercept HTTPS traffic to DoH providers. The fix is to leave browser DNS on automatic (the default), or disable the manually configured DoH provider. See [TROUBLESHOOTING.md §4 Browser DNS-over-HTTPS bypass](./TROUBLESHOOTING.md#browser-dns-over-https-doh-bypass) for diagnostic commands.
- Background tabs for SPAs (Reddit, YouTube) generate DNS traffic and consume quota even when the user is not actively browsing. The optional foreground-tab tracker described below sidesteps this for *measurement* (not for quota — the DNS-bucket signal still drives `DailyQuotaMinutes`).
- The 5-minute bucket slightly over-counts sessions shorter than 5 minutes (a 2-minute visit counts as 5 minutes) and slightly under-counts if another tool (AdGuard Home, systemd-resolved) intercepts queries before they reach Sentinel.

**Retention:** `usage.jsonl` is pruned once per calendar day to 60 days. The block event log (`events.jsonl`) is pruned to 30 days on the same tick.

### Foreground tab tracking (macOS, opt-in)

A second usage signal sits alongside DNS-bucket tracking for users who want the
"how long did I actually look at this site" view rather than "how often did the
machine resolve it." It's macOS-only and opt-in via `settings.enable_foreground_tracking`.

**How it works.** Each scheduler tick (1/min), if the flag is on, the scheduler
runs a single AppleScript probe that returns `frontmost_app<TAB>active_url<TAB>idle_seconds`.
Idle comes from IOKit's `HIDIdleTime` (no entitlement needed). The active URL
is read from `active tab of front window` for Chrome/Arc/Brave, and `current
tab of front window` for Safari. The tick is recorded as a `UsageEvent{Kind: "foreground"}`
in the same `usage.jsonl` only when **all** of these are true:

1. `idle_seconds < 60` — user is in front of the machine (not just leaving a tab open).
2. `frontmost_app` is one of Chrome / Safari / Arc / Brave Browser.
3. The URL parses as `http`/`https` with a non-empty host (filters out `chrome://newtab/`, `about:blank`, etc.).
4. The host (with `www.` stripped, lowercased, subdomain-aware) matches a domain in some configured group **other than `_doh`**.

The privacy floor — point 4 — is deliberate: the tracker only ever logs domains
the user has already configured for blocking. Visiting your bank or therapist
records nothing. `_doh` is excluded because those endpoints aren't user-visible
sites and the user explicitly asked us to scope the metric.

**Aggregation.** Foreground events are aggregated in **1-minute** buckets
(`TS.Unix() / 60`), one minute per distinct bucket. This is naturally
minute-granular because the scheduler ticks each minute and emits at most one
event per tick. DNS-kind events are still aggregated in 5-minute buckets
(`ComputeGroupUsageMinutes` filters by `IsDNSKind()`), so the two signals stay
independent.

**Backwards compatibility.** Pre-feature `usage.jsonl` entries have no `kind`
field. `UsageEvent.IsDNSKind()` treats empty `Kind` as `dns`, so existing logs
keep aggregating into `used_minutes` without any migration step.

**Cross-mode coverage.** Foreground tracking has no coupling to `enforcement_mode`
— it lives in the scheduler tick, not in the enforcer. Specifically:

| Mode | DNS-bucket `used_minutes` | Foreground `foreground_minutes` |
|------|---------------------------|----------------------------------|
| `hosts` | unavailable (no DNS proxy) | works |
| `dns` | works | works |
| `strict` | works | works |

So foreground tracking is the *only* per-domain time signal in `hosts` mode.

**Quota enforcement is unchanged.** `DailyQuotaMinutes` continues to drive off
the DNS-bucket signal. Routing foreground time into quota is a bigger product
decision that's deliberately out of scope for the metrics addition.

**Limitations.**
- macOS-only. Windows is tracked separately as a follow-up.
- Browser-only. Time spent in Slack, Discord, Xcode, etc. is not counted (tracked separately as a follow-up).
- 60-second granularity. Sessions shorter than a minute may not register.
- Inherits the `osascript` fragility tax of the per-tick close path — same `su -` user-context elevation under launchd-as-root.

### AppleScript abstraction

Two interfaces, both globally settable so tests can swap in stubs:

```go
type AppleScriptGenerator interface {
    GenerateWarningScript(domains []string) string
    GenerateCloseTabsScript(domains []string) string
}

type ScriptExecutor interface {
    ExecuteScript(script string) error
    LogScript(script string)
}
```

The default executor (`MacOSScriptExecutor`) writes the script to `/tmp/df_script.scpt` and runs it via `osascript`. If the daemon is running as root, it shells out as the console user (via `su - <user> -c osascript ...`) so the notification appears in the user's UI session and AppleScript can talk to Chrome/Safari. Console user is detected via `stat -f %Su /dev/console`.

The close-tabs script enumerates Chrome, Safari, Arc, and Brave windows, matches tab URLs against the blocked domain list (substring match), and closes matching tabs in all four browsers. The script also tracks a `closedCount` and emits a single bundled `display notification` ("Closed N tab(s) on ⟨domains⟩") at the end when `closedCount > 0` — close + notify happen in one atomic osascript invocation, so there is at most one fork/exec per tick.

The per-tick driver (`runPerTickCloseTabs`) gates the script on `getOpenBrowserDomains` first, so when no browsers are running or no tabs match, no close script is generated and no notification fires. The probe AppleScript itself is also guarded with `if application X is running` per browser, returning silently when none are open.

The warning script (3-minute pre-block) similarly checks `getOpenBrowserDomains` before firing a notification. This is why the warning UX isn't noisy: if you don't have YouTube open at 08:57, you don't get pinged about it at 09:00.

### Internal state

```go
var activeBlocks    = make(map[string]bool)  // last tick's blocked set
var lastEvalTime    time.Time
var lastWarningTime = make(map[string]time.Time)  // per-domain debounce
```

All guarded by `activeBlocksMu sync.RWMutex` and `lastWarningMu sync.Mutex`. The `GetStatus()` function returns a copy of `activeBlocks` plus `lastEvalTime` for the `/api/status` endpoint.

---

## 6. DNS proxy

`internal/proxy/dns.go`. Used only when `enforcement_mode` is `dns` or `strict`. Bypassed entirely in `hosts` mode.

### The pure function

```go
func GetDNSResponse(
    r *dns.Msg,
    blocked map[string]bool,
    primaryDNS, backupDNS string,
) (*dns.Msg, error)
```

For each query:
1. If the question name (suffix-trimmed) is in `blocked` and the qtype is `A`, answer `<name> 60 IN A 0.0.0.0` and return.
2. Otherwise forward via `dns.Client.Exchange` to `primaryDNS`. On error, retry with `backupDNS`. Return the upstream response verbatim.

This function is what the test suite hits — no port binding, no global state, just `(query, blocked-set, upstream-server)` → response.

### Subdomain matching

`IsDomainBlocked(domain, blocked)` checks both exact match and `.suffix` match — so `m.youtube.com` is blocked when `youtube.com` is in the set. This is `dns`-mode equivalent of the static prefix list that `HostsEnforcer` writes.

### The server

`StartDNSServer()` binds `127.0.0.1:53/udp`, registers `handleDNSRequest` for the `.` zone, and blocks. `handleDNSRequest` reads the current `blocked` map under a read-lock, then calls `GetDNSResponse`.

`StopDNSServer()` calls `dns.Server.Shutdown()` for graceful cleanup. The shutdown happens in `program.Stop()` in `main.go`.

`UpdateBlockedDomains(newBlocked)` is the only mutator — called by `DNSEnforcer.Activate/Deactivate/DeactivateAll` under its own mutex; the proxy then publishes the new map under its own write-lock. Two-tier locking keeps the per-query read lock on the hot path very cheap.

---

## 7. pf (Packet Filter) integration

`internal/pf/pf.go`. macOS-only, requires root, currently used only by `StrictEnforcer`. All exported functions are no-ops on non-darwin so the package can be imported unconditionally.

### Anchor file model

The daemon owns one pf anchor named `sentinel`, stored at `/etc/pf.anchors/sentinel`. The anchor file content is regenerated on every Activate. It has **two sections** depending on whether the `_doh` group is part of the active block set:

```
# Section 1 — regular blocked-domain IPs (all ports, both protocols)
block drop out quick inet  proto tcp from any to { 142.251.215.110 142.251.215.111 ... }
block drop out quick inet  proto udp from any to { 142.251.215.110 ... }
block drop out quick inet6 proto tcp from any to { 2607:f8b0:4007:80a::200e ... }
block drop out quick inet6 proto udp from any to { 2607:f8b0:4007:80a::200e ... }

# Section 2 — DoH/DoT endpoints, port-restricted so plain UDP/53 still works
block drop out quick inet  proto tcp from any to { 8.8.8.8 1.1.1.1 ... } port = 443
block drop out quick inet  proto tcp from any to { 8.8.8.8 1.1.1.1 ... } port = 853
block drop out quick inet  proto udp from any to { 8.8.8.8 1.1.1.1 ... } port = 853
# (plus inet6 equivalents)
```

macOS automatically promotes inline IP lists to internal `__automatic_<hash>_<n>` tables, which is what shows up under `pfctl -a sentinel -s Tables`. The two sections exist because:

- Regular blocked IPs (Reddit, YouTube, …) get an **all-port** drop — there's no legitimate traffic the daemon needs to those CDNs.
- DoH/DoT endpoints (`dns.google`, `cloudflare-dns.com`, …) get a **port-restricted** drop on TCP/443 and TCP+UDP/853 only. UDP/53 plain DNS to the same IPs is left open so the daemon's own `backup_dns` (typically `1.1.1.1:53`) keeps working — which is what makes the `_doh` group safe to enable by default in strict mode.

The two-section split is what allows the `_doh` group to be **always-on** in strict mode without breaking the daemon's own upstream resolution. See `internal/pf/pf.go` (`Generate*` helpers) and the rendered example at `/etc/pf.anchors/sentinel` after `sudo ./sentinel start`.

The anchor is wired into `/etc/pf.conf` between marker lines:

```
# sentinel:begin
anchor "sentinel"
load anchor "sentinel" from "/etc/pf.anchors/sentinel"
# sentinel:end
```

`InstallAnchor()` writes a stub anchor file, injects the pf.conf block (idempotent — checks for the marker first), runs `pfctl -n -f /etc/pf.conf` to validate the config in dry-run mode, then `pfctl -f /etc/pf.conf` to load it, and finally `pfctl -e` to enable pf if it isn't already.

`RemoveAnchor()` flushes the table, strips the marker block from pf.conf, reloads pf, and deletes the anchor file.

`ActivateBlockMixed(domains, dohDomains, primaryDNS, backupDNS)` resolves each domain to A and AAAA addresses against both `primaryDNS` and `backupDNS` (the union widens coverage when CDN edges return geo-different answers per resolver), regenerates the anchor file with the two sections shown above, runs `pfctl -a sentinel -f <anchor>` to load it, and then runs `pfctl -k <src> -k <ip>` per IP **only for the regular block set** to kill any existing connections. State-kill is intentionally skipped for DoH IPs: killing all states to `1.1.1.1` would also drop the daemon's own `backup_dns` sessions on UDP/53. `ActivateBlock(domains, primaryDNS, backupDNS)` is a thin wrapper that passes `nil` for `dohDomains`.

`DeactivateBlock()` writes a stub anchor (`# no IPs to block`) and reloads it, which empties any `__automatic_*` tables that pf had promoted from the previous activation. No explicit `pfctl -t … -T flush` is needed because the reload replaces the rule set wholesale.

`Refresh()` is called by `StrictEnforcer` every scheduler tick to re-resolve the currently-blocked domains. CDNs rotate IPs constantly, so the rule set has to be rebuilt periodically rather than left static after the initial activation.

### Why preview functions are exported

The web UI's `/api/pf-preview` endpoint calls `pf.GeneratePreview(domains, dnsServer)` to show users what *would* happen in strict mode without touching pf. This is essentially `ActivateBlock` minus all the side effects — pure resolution and content generation.

---

## 8. Configuration

`internal/config/config.go`. Single global `AppConfig` guarded by an `RWMutex`. All access goes through `GetConfig()` (returns a value copy) and `LoadConfig()` / `SaveConfig()` (file I/O under exclusive lock).

### Schema

```go
type Config struct {
    Settings Settings            `json:"settings"`
    Groups   map[string][]string `json:"groups"`
    Rules    []Rule              `json:"rules"`
    Pause    *PauseWindow        `json:"pause,omitempty"`
    Pomodoro *PomodoroSession    `json:"pomodoro,omitempty"`
}

type Settings struct {
    PrimaryDNS               string `json:"primary_dns"`
    BackupDNS                string `json:"backup_dns"`
    AuthToken                string `json:"auth_token"`
    EnforcementMode          string `json:"enforcement_mode,omitempty"`           // "hosts" | "dns" | "strict"
    DNSFailureMode           string `json:"dns_failure_mode,omitempty"`           // "open" (default) | "closed"
    EnableForegroundTracking bool   `json:"enable_foreground_tracking,omitempty"` // macOS-only opt-in
}

type Rule struct {
    Group             string                `json:"group"`                        // key into Config.Groups
    IsActive          bool                  `json:"is_active"`
    DailyQuotaMinutes int                   `json:"daily_quota_minutes,omitempty"` // 0 = no quota
    Schedules         map[string][]TimeSlot `json:"schedules"`                    // weekday → slots
}

type TimeSlot struct {
    Start string `json:"start"`  // "HH:MM"
    End   string `json:"end"`    // "HH:MM"
}

type PauseWindow struct {
    Until time.Time `json:"until"`
}

type PomodoroSession struct {
    Phase        string    `json:"phase"`           // "work" | "break"
    PhaseEndsAt  time.Time `json:"phase_ends_at"`
    WorkMinutes  int       `json:"work_minutes"`    // 1..120, captured at session start
    BreakMinutes int       `json:"break_minutes"`   // 1..60, captured at session start
}
```

`Config.ResolveGroup(name)` returns the domain list for `name`, or `nil` if the group is missing — the scheduler treats a missing group as a silent no-op so a rule referencing a deleted group degrades safely instead of panicking.

`Settings.GetEnforcementMode()` returns the validated mode, defaulting to `"hosts"` for empty or unrecognised values. This is what allows a pre-1.x config without the `enforcement_mode` field to upgrade transparently.

### Why groups instead of inline domains

A rule used to carry a single `Domain` string, which meant blocking five gaming sites required five rules with the same schedule duplicated five times. The groups indirection lets one schedule cover an arbitrary set of domains, and means edits to "what counts as a game" don't require touching any schedule. `EvaluateRulesAtTime` and `CheckWarningDomainsAtTime` resolve the group at evaluation time (per tick), so live edits to a group's member list propagate within 60 seconds without any rule changes. There is no inline-domains fallback — `ValidatePostedConfig` rejects any rule whose `group` doesn't reference an existing key.

### File location

| OS | Path |
|---|---|
| macOS | `/Library/Application Support/Sentinel/config.json` |
| Windows | `%PROGRAMDATA%\Sentinel\config.json` |
| Linux | `/etc/distractionsfree/config.json` |
| `UseLocalConfig=true` | `./config.json` |

`UseLocalConfig` is a package-level boolean set by `--local`, `--test-web`, and `--test-query` so they can run against a working-directory config without touching system paths.

### Bootstrap

On first run, if the config file doesn't exist, `LoadConfig()` writes a default config:

- `primary_dns: "8.8.8.8:53"`, `backup_dns: "1.1.1.1:53"`
- `enforcement_mode: "hosts"`
- `dns_failure_mode: "open"`
- `enable_foreground_tracking: false`
- A randomly generated 32-character hex `auth_token`
- Two seed groups — `games` (roblox/epic/steam/fortnite/minecraft) and `social` (discord/facebook/instagram/tiktok/snapchat/reddit) — each bound by an active rule to the school window (09:00–15:00 weekdays) plus a nightly window (21:30–23:59 every day).

If an existing config has a missing `auth_token`, one is generated and the file is rewritten. This is the only field auto-modified on load.

### Reload cadence

The scheduler calls `LoadConfig()` once per tick. `web.ConfigHandler` also calls it on every `GET /api/config` so the dashboard always sees the current file state, not a startup snapshot. There is no inotify/FSEvents listener — 60 seconds of staleness is acceptable for human-edited rules.

### Pause window

`PauseWindow.Until` is an absolute `time.Time` (RFC3339 in JSON). `Config.IsPaused(t)` returns true when the field is non-nil and `t < Until`. `EvaluateRulesAtTime` and `CheckWarningDomainsAtTime` both short-circuit to empty when paused. The scheduler self-clears expired pauses at the top of each tick so `config.json` doesn't accumulate stale entries.

### Pomodoro session

Pomodoro is the inverse of pause: where pause forces blocking *off*, the work phase of a Pomodoro session forces blocking *on* for every active rule, regardless of schedule. `Config.IsLockedByPomodoro(t)` returns true when `Pomodoro != nil`, `Phase == "work"`, and `t < PhaseEndsAt`. When that returns true, `EvaluateRulesAtTime` short-circuits *before* the per-rule schedule check and returns the union of every active rule's resolved domains.

The session is captured in config (rather than held in scheduler memory) so it survives daemon restarts and laptop sleep — the next tick after wake re-runs the same `IsLockedByPomodoro` check and either keeps the lock, transitions to break, or clears the session.

Phase transitions happen in the scheduler tick when `t >= PhaseEndsAt`:

1. **work → break** — `config.AdvancePomodoroPhase()` rewrites the field with `Phase="break"` and `PhaseEndsAt = now + BreakMinutes`. A macOS notification fires (`"Sentinel — Break time!"`).
2. **break → cleared** — `config.ClearPomodoro()` sets the field to `nil`. Notification: `"Sentinel — Ready?"`. Sessions don't auto-restart; the user must explicitly start the next one from the dashboard.

Two endpoints layer extra protection on top of this: the work phase causes `POST /api/pomodoro` (start) to refuse if a session is already running, `DELETE /api/pomodoro` to return `423 Locked`, and `POST /api/config/update` to also return `423`. This makes "edit your way out of focus mode" require dropping to `config.json` directly with admin access — friction, not security.

---

## 9. Web server & HTTP API

`internal/web/server.go`. Listens on `127.0.0.1:8040`. Two entry points:

- `StartWebServer()` — used in service mode; same routes, called by `main.go`.
- `StartTestWebServer()` — used in `--test-web` mode; loads config from working dir, otherwise identical.

### Routes

| Path | Method | Auth | Purpose |
|---|---|---|---|
| `/` | GET | — | Embedded SPA (HTML/CSS/JS) |
| `/api/config` | GET | none | Current config — public so the UI can bootstrap the auth token |
| `/api/config/update` | POST | token | Validate and replace config |
| `/api/status` | GET, POST | token | `{blocked_domains, last_evaluated, enforcement_mode, paused, paused_until, quotas[]}` |
| `/api/test-query?time=&domain=` | GET, POST | token | Evaluate `(time, domain)`; POST a `config` form field to test against a custom config |
| `/api/hosts-preview` | GET, POST | token | Show `/etc/hosts` lines for the current blocked set without writing |
| `/api/pf-preview` | GET, POST | token | Show resolved IPs and pf anchor content (strict mode only) |
| `/api/pause` | POST | token | Body `{"minutes": N}` — `1 <= N <= 240` |
| `/api/pause` | DELETE | token | Clear pause |
| `/api/pomodoro/start` | POST | token | Body `{"work_minutes": 1..120, "break_minutes": 1..60}` |
| `/api/pomodoro` | DELETE | token | Clear session — returns `423` during the work phase |
| `/api/usage` | GET | token | Per-group and per-domain DNS usage minutes; `?range=today\|7d\|30d\|60d` |

### Auth model

`authMiddleware` rejects any `/api/*` request whose `X-Auth-Token` header doesn't match `config.Settings.AuthToken`. **`GET /api/config` is intentionally exempt** so the SPA can fetch the token on first load and hold it in memory for subsequent calls. The token is treated like any other secret in the config file.

This is local-only auth — the server binds `127.0.0.1`, never `0.0.0.0`. Anything running on the same machine as your user can still read the config file and impersonate the dashboard. The auth token's job is to distinguish "the dashboard you opened" from "the random local app that decided to fiddle with port 8040", not to defend against a determined local attacker.

### Validation

`ValidatePostedConfig(cfg)` runs server-side on every `POST /api/config/update`. Checks:

- `enforcement_mode` is empty or one of `"hosts"`, `"dns"`, `"strict"`.
- Every group has a non-empty name and at least one non-empty domain.
- Every rule has a non-empty `group`, and that group exists in `cfg.Groups`.
- Every schedule key is a valid weekday name.
- Every weekday's slot list is non-empty.
- Every slot has a valid `15:04` `Start` and `End`.
- `Start < End`.

The browser-side `parseAndValidate` mirrors the same rules so the textarea flags errors before they reach the server. Both are intentional duplicates — the JS catches typos in the editor; the Go is the source of truth.

The auth token is preserved across updates regardless of what the client posts — so the dashboard can't accidentally rotate the secret out from under itself.

### The `resolveConfig` helper

`/api/test-query`, `/api/hosts-preview`, and `/api/pf-preview` all support an optional posted config: if the request has a JSON body it's used as the config to evaluate against, otherwise the disk config is reloaded and used. This is what makes the **Test** tab able to "what-if" an arbitrary config without committing it.

### Embedded assets

`//go:embed static/*` bundles `static/index.html` (a single-page app: HTML + inline CSS/JS) into the binary. The static handler serves it via `http.FileServer(http.FS(fsys))`.

---

## 10. Process lifecycle

`cmd/app/main.go` is both CLI dispatcher and `service.Service` implementation.

### CLI dispatch order

```
1. setup                         → copy binary, service install + start, exit
2. clean [--yes]                 → cleanup.* steps, exit
3. --test-web                    → web.StartTestWebServer (UseLocalConfig=true)
4. --test-query <t> <d>          → testcli.QueryBlocking, exit
5. --test-applescript            → run AppleScript demo, exit
6. --local                       → run program.run() in foreground (UseLocalConfig=true)
7. --set-mode <mode>             → config.SetEnforcementMode(mode) + SaveConfig, exit
8. install/uninstall/start/stop/status/run → service.Control(s, arg)
9. (no args)                     → s.Run() — service supervisor mode
```

### Service start path

`program.Start(s)` is called by the service framework. It spawns a goroutine running `program.run()` and returns immediately so the supervisor doesn't think the service hung.

```go
func (p *program) run() {
    config.LoadConfig()
    cfg := config.GetConfig()
    mode := cfg.Settings.GetEnforcementMode()

    e := enforcer.New(cfg)
    e.Setup()
    p.enforcer = e

    scheduler.SetEnforcer(e)
    scheduler.Start()
    go web.StartWebServer()

    if mode == "dns" || mode == "strict" {
        proxy.StartDNSServer()   // blocks until shutdown
    } else {
        select {}                // hosts mode: park the goroutine
    }
}
```

The terminating call differs by mode: in `dns`/`strict` the DNS server holds the goroutine (and is what `program.Stop` shuts down); in `hosts` mode there's no port binding so we `select {}` to keep the goroutine alive.

### Service stop path

```go
func (p *program) Stop(s service.Service) error {
    proxy.StopDNSServer()
    if p.enforcer != nil {
        p.enforcer.Teardown()
    }
    return nil
}
```

`Teardown` is what restores the system to a usable state:

- `HostsEnforcer.Teardown` → `DeactivateAll` → strip the managed block from `/etc/hosts`.
- `DNSEnforcer.Teardown` → reset Wi-Fi DNS, flush DNS cache.
- `StrictEnforcer.Teardown` → DNS teardown + `pf.RemoveAnchor`.

`DNSEnforcer.Teardown` only resets the `Wi-Fi` interface today (issue #12). For multi-interface cleanup, use `clean`.

---

## 11. Cleanup (`clean`)

`internal/cleanup/cleanup.go` plus `priv_unix.go` / `priv_windows.go`. The forensic recovery path: undo every system change sentinel might have made, even if the service crashed mid-write.

Each cleanup action is a `Step` with a status (`done`/`skipped`/`warn`/`error`) and an optional `Critical` flag. The summary is printed line-by-line at the end. Critical failures cause a non-zero exit code.

### Steps, in order

1. **Stop the running service** (`service.Control(s, "stop")`). "Not running" is a warning, not an error.
2. **Reset DNS on every interface** that points at `127.0.0.1`. On macOS this enumerates `networksetup -listallnetworkservices` and runs `networksetup -getdnsservers <name>` per interface. Only interfaces that match `127.0.0.1` are reset to `Empty`. On Windows: a one-liner PowerShell `Get-DnsClientServerAddress | Where ServerAddresses -contains "127.0.0.1" | Set-DnsClientServerAddress -Reset`.
3. **Strip the managed block from `/etc/hosts`** via `HostsEnforcer.DeactivateAll`. Idempotent — no-op if no block is present.
4. **Remove the pf anchor** via `pf.RemoveAnchor`. Skipped on non-darwin or if the anchor file doesn't exist.
5. **Flush the DNS cache** (`dscacheutil -flushcache` + `killall -HUP mDNSResponder` on macOS; `ipconfig /flushdns` on Windows).
6. **Uninstall the system service** (`service.Control(s, "uninstall")`). Critical.
7. **Remove the config directory** (`config.ConfigDir()`). Prompts unless `--yes` is passed.
8. **Remove temp files** (currently `/tmp/df_script.scpt`).
9. **Verify port 53 is free** by attempting a UDP bind to `127.0.0.1:53`. If something else is holding it, emit a warning suggesting `lsof -i :53`.

### Privilege check

`IsPrivileged()` is built per-OS:

- Unix: `os.Geteuid() == 0`.
- Windows: token-based check via `golang.org/x/sys/windows` against `SECURITY_BUILTIN_DOMAIN_RID + DOMAIN_ALIAS_RID_ADMINS`.

`runClean` exits early with a clear error message if not privileged.

---

## 12. Testability strategy

The principle: every piece of logic that's worth testing is a **pure function** of its inputs. Side-effecting code is the thinnest possible wrapper around those functions.

| Pure function | Tested behaviour |
|---|---|
| `scheduler.EvaluateRulesAtTime(t, cfg)` | All rule semantics: weekday, time slot, multi-slot, inactive rule, paused config, edge times. |
| `scheduler.CheckWarningDomainsAtTime(t, cfg)` | The 3-min pre-block window. |
| `proxy.GetDNSResponse(query, blocked, primary, backup)` | Block-returns-0.0.0.0, allowed-forwarded, primary-failover, exact qtype handling, suffix subdomain matching. |
| `proxy.IsDomainBlocked(domain, blocked)` | Exact and suffix matches. |
| `enforcer.GenerateHostsEntries(domains)` | Hosts-line generation for the static prefix list. |
| `pf.GenerateAnchorContent(ips)` | pf table syntax, empty-list behaviour. |
| `pf.GeneratePreview(domains, dnsServer)` | Resolution + anchor content together. |
| `web.ValidatePostedConfig(cfg)` | All the bad-input paths. |
| `web.ConfigHandler` / `TestQueryHandler` etc. | HTTP shape via `httptest`. |
| `testcli.GetQueryResult(...)` | The struct that drives both the CLI output and the web UI. |

The DNS-response test deliberately queries real `8.8.8.8` and `1.1.1.1` upstreams to verify the forwarding path end-to-end. This is the only test that requires network access.

What's **not** tested by the suite (validated by hand on a real macOS box):

- Port 53 binding (`StartDNSServer`).
- `networksetup`, `dscacheutil`, `killall mDNSResponder`.
- `pfctl` invocations.
- `osascript` invocations against running Chrome/Safari.
- launchd / Windows Service Manager registration.

The `--test-web`, `--test-query`, and `--test-applescript` flags exist exactly to exercise these by-hand paths interactively.

### AppleScript test seam

The scheduler exposes `ScriptExecutor` and `AppleScriptGenerator` interfaces with package-level globals (`scriptExecutor`, `scriptGenerator`) and getters/setters. Tests inject a `TestScriptExecutor` that records executed scripts instead of running `osascript`. The interface boundary lets us assert on script *content* without touching the shell.

---

## 13. Concurrency model

Three long-lived goroutines in service mode:

1. **Scheduler tick loop** — single goroutine, fires `evaluateRules()` on a 1-min ticker.
2. **DNS server** — one goroutine for `Accept`, plus per-request goroutines spawned by `miekg/dns`.
3. **HTTP server** — one goroutine per request via `net/http` defaults.

### Shared state and locks

| State | Owner | Lock |
|---|---|---|
| `config.AppConfig` | `internal/config` | `sync.RWMutex mu` — read on `GetConfig`, write on `LoadConfig`/`SaveConfig`/`SetEnforcementMode`/`SetPause`/`ClearPause` |
| `scheduler.activeBlocks` | scheduler | `sync.RWMutex activeBlocksMu` — read on diff computation and `GetStatus`, write on tick commit |
| `scheduler.lastWarningTime` | scheduler | `sync.Mutex lastWarningMu` — only touched once per minute |
| `proxy.blockedDomains` | proxy | `sync.RWMutex blockMu` — read on every DNS request, write on `UpdateBlockedDomains` |
| `DNSEnforcer.blocked` | enforcer (one per process) | `sync.Mutex mu` — Activate/Deactivate batches |

The hot path is the per-DNS-query read of `proxy.blockedDomains`; that's why it's an `RWMutex` rather than a regular `Mutex`. Updates from the scheduler happen at most once per minute, so write contention is negligible.

`config.GetConfig()` returns a value copy of the `Config` struct. Slice and map fields inside (`Rules`, `Schedules`) are still references to shared memory — but nothing in the codebase mutates them through a `GetConfig` result. Mutations always go through `LoadConfig`/`SaveConfig`/`SetEnforcementMode`/`SetPause`.

---

## 14. Security model

Sentinel is **not** a security tool. It's a friction tool against your own future self. Treat it accordingly:

- The dashboard binds `127.0.0.1` only. Network attackers cannot reach `:8040`.
- The auth token in `X-Auth-Token` keeps random local processes from poking the API by accident, but anything running as your user can read `config.json` and impersonate the dashboard. Same for the PIN — it's client-side only.
- The Manage-tab PIN is not a password. It's a friction layer designed to make you pause and think before disabling your own focus rules. Trivially bypassed by anyone who reads the JS.
- The service runs as root (macOS launchd daemon, Windows Service). Anything that compromises the binary inherits root. Build releases via the GitHub Actions workflow; don't run a binary you didn't compile or download from a release tag.
- `/etc/hosts` and `/etc/pf.conf` edits use atomic temp-file + rename. A crash mid-write cannot corrupt the file. Markers (`# sentinel:begin`/`:end`) mean other tools that respect them won't trample our entries.
- The `clean` command is the canonical recovery path. It iterates every interface, does not assume a happy-path service stop succeeded, and exits non-zero if any critical step fails.

---

## 15. Platform-specific notes

### macOS

- Service framework: `launchd`. The `kardianos/service` library writes a plist to `~/Library/LaunchAgents/com.github.sentinel.plist` (or system equivalent depending on install context).
- `osascript` is invoked as the console user (resolved via `stat -f %Su /dev/console`) so notifications appear in the user's UI session and AppleScript can talk to Chrome/Safari. Running `osascript` as root produces a notification nobody can see.
- `networksetup` is the only supported way to set system DNS — there's no clean API.
- `dscacheutil -flushcache` + `killall -HUP mDNSResponder` is the canonical cache-flush incantation. Both are needed.
- `pfctl` requires root and a valid `/etc/pf.conf`. The pf integration validates with `pfctl -n -f` before applying with `pfctl -f`.

### Windows

- Service framework: Windows Service Manager via `kardianos/service`.
- Tab closing and pre-block notifications are not implemented — Windows has no equivalent of the AppleScript path. Could be done with PowerShell + browser automation, but isn't currently.
- `pf` is macOS-only (BSD packet filter); Windows support for strict mode would require WFP integration and isn't planned.
- DNS reset is done by PowerShell: `Set-DnsClientServerAddress -InterfaceAlias 'Wi-Fi' -ResetServerAddresses`.
- `ipconfig /flushdns` for cache flush.

### Linux

- Compiles, but the service path is untested; `kardianos/service` supports systemd but the codebase has no systemd-specific work.
- `hosts` mode works via `/etc/hosts`. `dns` mode works via `127.0.0.1:53`. AppleScript and pf paths are no-ops.
- DNS reset is not implemented for Linux interfaces.

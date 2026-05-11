# Session Log

Development history for this project, captured from Claude Code sessions. Ordered chronologically.

---

## Apr 26 — Session 1: README Badges & GitHub Pages Polish
**Session ID:** `a1d34ba6` · **Duration/Size:** ~786KB · **73 tool calls**

**Opening prompt:**
> "can you add appropriate badges like release version, build passing etc. may be last release date, if appropriate."

**What happened:**
- Added GitHub badges (latest release, build status, last release date) to README.
- Hit an issue where the CI badge was showing "failing" despite the workflow passing — investigated the badge URL and fixed it to point at the correct workflow name.
- Added download links for the latest released macOS and Windows binaries to the GitHub Pages site (hacker theme), using the GitHub Releases API URL pattern.
- Investigated how the hacker theme renders download buttons (via `_config.yml`) and wired up the links accordingly.
- Incorporated `img/logo.png` into the appropriate spots in the GitHub Pages layout so the logo renders on the site.
- Raised and merged a PR for each batch of changes.

**Wrap-up:** README badges working and accurate; GitHub Pages now shows logo, download buttons for macOS and Windows binaries pointing at latest release.

---

## Apr 26 — Session 2: Product Messaging & Naming
**Session ID:** `5a7827bf` · **Size:** ~352KB · **27 tool calls**

**Opening prompt:**
> "I want the main initial content to be tailored for general productivity users, parents concerned about children's online time. But, I want to keep the technical content somewhere down the line because that's what distinguishes this from all other similar tools."

**What happened:**
- Rewrote the top of the landing content to lead with productivity/parental use cases rather than technical implementation details; technical depth moved to a later section.
- Discussed dropping the hyphen in "Distraction-Free" as a product name — explored alternatives.
- Iterated on names: considered "Sentinal" (user's initial lean), discussed tone — wanted something non-negative.
- Landed on **Sentinel** with tagline **"No Distractions"**.
- Raised the question of whether to rename the executable too, and decided to treat that as a follow-on.

**Wrap-up:** Product name settled as Sentinel. Content hierarchy updated — user-friendly framing first, technical depth second. Executable rename deferred to next session.

---

## Apr 26 — Session 3: Full Rename to Sentinel + MIT License
**Session ID:** `80c4d585` · **Size:** ~1809KB · **121 tool calls**

**Opening prompt:**
> "We are renaming the executable to Sentinel. The binary commands (`./distractions-free`), repo URLs, and system paths need to be changed along with matching docs updates. can you make the changes"

**What happened:**
- Performed a full rename across the codebase: binary name, Go module path, service name (`com.sentinel`), system paths (`/Library/Application Support/Sentinel/`), all docs and README references.
- Clarified that the GitHub repo itself needed to be renamed manually — Claude can't do that via API, so user was informed.
- Fixed `_config.yml` for GitHub Pages to reflect the new name/URLs.
- Raised and merged the rename PR.
- Identified and fixed a logo transparency issue on the GitHub Pages site (PNG was rendering with a white background instead of transparent).
- User asked for a suitable open-source license — explored options, landed on **MIT License**, added `LICENSE` file.
- Raised and merged a second PR for the logo and license changes.

**Wrap-up:** Project fully renamed to Sentinel across all code, config, paths, and docs. MIT license added. Logo rendering fixed.

---

## Apr 26 — Session 4: Release v0.1.0
**Session ID:** `dababf02` · **Size:** ~22KB · **2 tool calls**

**Opening prompt:**
> "can you release v0.1.0"

**What happened:**
- Tagged and pushed `v0.1.0`, triggering the release workflow to build and publish binaries.

**Wrap-up:** First public release shipped.

---

## Apr 28 — Session 5: Default Config Overhaul + Single Config Source
**Session ID:** `ef31ccdd` · **Size:** ~431KB · **43 tool calls**

**Opening prompt:**
> "make some changes to the default config. to games add rbxcdn.com. add a new category and include youtube and similar common domains. games should be blocked weekdays 9-12, 2-4, 6-7. same for videos and for social block full time. weekends, games blocked 9-12, 6-7. same for videos and social block full time."

**What happened:**
- Updated default config: added `rbxcdn.com` to Games, created a new Videos category (YouTube and similar), defined Social as always-blocked.
- Blocking schedule set: weekdays Games/Videos blocked 9-12, 2-4, 6-7; Social blocked all day; weekends Games/Videos blocked 9-12, 6-7.
- Discovered the default config was duplicated in three places — discussed why and whether it could be unified.
- Consolidated to a single canonical config source; service install path now reads from that.
- Cleaned up the auth token: left the field empty in the file, load logic fills it at startup.
- Committed config changes and corresponding README updates as separate commits on a feature branch.
- Raised PR, approved, and cut the next release.

**Wrap-up:** Default config now meaningful out of the box. Config duplication resolved. Next version released.

---

## Apr 29 — Session 6: Cross-Midnight Time Ranges
**Session ID:** `6bdb38e6` · **Size:** ~351KB · **25 tool calls**

**Opening prompt:**
> "I am thinking, if we want to block a domain overnight, say from 9:30pm to next day 8:00am, currently it needs two entries from 21:30-23:59 and 00:00-08:00. I want to think about supporting specifying the same as 21:30-08:00 and make it work accordingly. Think about it and show me a plan."

**What happened:**
- Discussed the UX problem: overnight blocks requiring two separate schedule entries is awkward.
- Agent explored the scheduler logic and produced an implementation plan for cross-midnight range detection (if `end < start`, treat as spanning midnight).
- Reviewed the plan, approved implementation.
- Changes implemented, committed, PR raised, merged, and a new release cut.

**Wrap-up:** Scheduler now supports single cross-midnight time range entries (e.g., `21:30-08:00`). Released.

---

## Apr 29 — Session 7: Simplified Install/Uninstall
**Session ID:** `42c9cf03` · **Size:** ~433KB · **39 tool calls**

**Opening prompt:**
> "can you suggest any better and simpler ways to install and uninstall this app/service? including cleanup. right now, it takes three different commands to install for instance."

**What happened:**
- Explored the current install flow (three separate commands required) and identified friction.
- Agent proposed and designed a simplified `install`/`uninstall` single-command flow.
- Introduced `dev-install` and `dev-uninstall` as separate targets for development use — user asked for a clear explanation of when to use dev vs production variants.
- Confirmed: `dev-*` uses local binary + local config, skips system path setup; production `install` goes through the full launchd service setup.
- Implemented changes on a feature branch, raised PR, merged, and released.

**Wrap-up:** Installation reduced to a single command. Dev and production install paths clearly separated. Released.

---

## Apr 29 — Session 8: CLI Argument Review & Consistency
**Session ID:** `2c98a133` · **Size:** ~633KB · **75 tool calls**

**Opening prompt:**
> "can you review all the command line arguments and tell me if they are good, well-named and consistent? Tell me if you would recommend any changes. Also, let me know if -- prefix should be used or dropped."

**What happened:**
- Reviewed all existing CLI flags for naming clarity, consistency, and Go CLI conventions.
- Recommended keeping `--` prefix (standard for long flags in Go), renaming several flags for clarity and consistency.
- Implemented the agreed renaming and consistency fixes across the codebase and docs.
- Raised PR, merged, released next version.

**Wrap-up:** CLI flags standardised with `--` prefix and consistent naming. Released.

---

## Apr 30 — Session 9: Product Landing Page
**Session ID:** `00faa62e` · **Size:** ~629KB · **57 tool calls**

**Opening prompt:**
> "just what we can do to have a product page in a better way, right now using a simple github pages. is it possible to do something more elaborate like say astrowind. Use this site only for format/style etc. I don't need to or want to use Astro. Just propose a plan. Note, if we are adding a lot of new files, plan such that they are properly located in a separate folder."

**What happened:**
- Explored the existing simple GitHub Pages/hacker theme setup.
- Planned a richer landing page (pure HTML/CSS/JS, no framework) inspired by SaaS landing page conventions — hero, features, install section, download CTAs — living in a `docs/` folder.
- Discussed local testing: run Jekyll locally or open the HTML directly.
- Built out the full landing page per the plan.
- Identified that macOS was not prominently featured — user wanted macOS (especially Apple Silicon) to be the primary CTA, Windows secondary.
- Fixed a button text contrast issue (text was same colour as background).
- Raised PR, merged, released.
- Added the Sentinel logo/icon to the web admin/status dashboard page as a follow-on.
- Investigated the missing macOS Intel build in the release workflow matrix — fixed so the workflow actually runs that target.

**Wrap-up:** Full product landing page live on GitHub Pages. macOS Apple Silicon CTA prominent. macOS Intel build fixed in CI. Logo in admin dashboard. Released.

---

## Apr 30 — Session 10: Feature Review & GitHub Issue Creation
**Session ID:** `027fb157` · **Size:** ~361KB · **42 tool calls**

**Opening prompt:**
> "review the functionality and feature set for this product and give me any ideas on any improvements or additional features. what else would other such products do that we should add."

**What happened:**
- Agent explored the full codebase (entry point, config, scheduler, proxy, web dashboard, test CLI).
- Generated a prioritised list of feature ideas and improvements spanning: UX, reliability, cross-platform parity, observability, and power-user features.
- Created GitHub issues for each item — both high-priority and lower-priority — with labels for priority, complexity, and type (fix/feature/enhancement).
- Added a note to each issue that the write-up is initial guidance and the implementer should revalidate before starting, since the codebase may have diverged.
- Added comments to issues that would also require README or docs site updates, flagging that documentation must be part of the implementation.

**Wrap-up:** Backlog fully populated with labelled GitHub issues. All issues include a revalidation note and docs-update flag where relevant.

---

## Apr 30 — Session 11: Bulk Fix — Issues 41, 42, 43, 47, 52
**Session ID:** `80332093` · **Size:** ~926KB · **100 tool calls**

**Opening prompt:**
> "handle issues of fix type 41, 42, 43, 47 and 52. each issue in a separate feature branch, with a good pr and commit messages. work in this sequence and ask me after raising pr if it should be approved and merged before proceeding to the next."

**What happened:**
- Agent fetched all five issue descriptions and explored the relevant codebase areas.
- Each issue was implemented on its own feature branch with a focused commit and descriptive PR.
- Issues handled sequentially: after each PR was raised, user was asked to approve before proceeding.
- User approved merges one by one; Claude handled branch cleanup after each merge.
- Versioning discussion mid-session (user confirmed to use the next version number).
- After completing, user asked whether to continue in this session or `/clear` first for the next batch — deferred to next session.

**Wrap-up:** Five fix issues resolved, each with individual PRs, merged and cleaned up. Released.

---

## May 1 — Session 12: Issues 63 & 64 — Port 53 Conflicts & DNS Crash Handling
**Session ID:** `e078b7b6` · **Size:** ~916KB · **86 tool calls**

**Opening prompt:**
> "review and fix issues 63 and 64. Write detailed explanation root cause and fix details in the issue and describe in PR. separate commits for both fixes."

**What happened:**
- Reviewed both issues: #63 was about graceful handling when port 53 is already in use; #64 was related DNS crash/error behaviour.
- Fixed #63: error message on port 53 conflict revised — detailed steps removed from the runtime log message.
- Moved the troubleshooting content (what to do if running AdGuard Home, etc.) into `troubleshooting.md` instead of embedding it in error output.
- Fixed #64 with a separate commit.
- Each issue got a detailed root cause write-up posted back to the GitHub issue, and PRs had full descriptions.
- Both PRs merged, branches cleaned up, new release cut.

**Wrap-up:** Port conflict and DNS crash issues resolved. Troubleshooting guidance properly located in `troubleshooting.md`. Released.

---

## May 1 — Session 13: DNS Failure Mode Config Option
**Session ID:** `f38a5726` · **Size:** ~757KB · **73 tool calls**

**Opening prompt:**
> "as documented in troubleshooting.md if there are port conflicts due to adguard, we are asking the user to change adguard port. Instead, I am wondering if Sentinel can use a different port say 5353 that is less likely to conflict with others? And default the upstream dns to be whatever was previously configured? Are there any considerations or tradeoffs with this approach?"

**What happened:**
- Discussed using port 5353 as a default to avoid conflicts — identified tradeoffs (requires updating system DNS resolver to point at non-standard port; some resolvers don't support this).
- Follow-up question: what happens if Sentinel crashes after it has changed the system DNS to point at itself? Answer: DNS resolution fails completely until Sentinel restarts or DNS is manually restored.
- Decided to expose a `dns_failure_mode` config option: `safe` (default — backup DNS is the original system DNS, so if Sentinel crashes, resolution falls back gracefully) vs `strict` (fail hard, user opt-in).
- Also auto-detected upstream DNS and auto-configured system DNS on startup (separate PR from the prior session's work).
- Confirmed that `stop`/`clean` commands properly restore the backup DNS when cleaning up.
- Raised PR for the `dns_failure_mode` feature, ensured prior pending changes were committed and PR'd first.
- Released next version.

**Wrap-up:** `dns_failure_mode` config option added. System DNS auto-detection on startup. Cleanup commands verified to restore DNS. Released.

---

## May 1 — Session 14: Log Rotation & Storage (Issue 44)
**Session ID:** `59ad7a53` · **Size:** ~376KB · **43 tool calls**

**Opening prompt:**
> "review issue 44. How do we make sure this log file doesn't grow unbounded? and what is a good place to store it? same as config file?"

**What happened:**
- Fetched issue #44 details and explored the current logging setup and config file path conventions.
- Discussed log rotation strategies: size-based rotation with a configurable max size and number of retained files.
- Agreed that storing the log file alongside the config (`/Library/Application Support/Sentinel/`) is the right location for consistency.
- Implementation of structured log rotation was planned/started.
- Session ended mid-work (user asked "do you remember my last question" — context had been lost). Likely continued in a subsequent session.

**Wrap-up:** Log rotation design agreed (size-based, co-located with config). Implementation may have carried over.

---

## May 1 — Session 15: This Session Log
**Session ID:** `79011b10` · **Size:** ~69KB · **6 tool calls**

**Opening prompt:**
> "can you list all the sessions in this folder along with timestamps, any summary stats and the prompts I used."

**What happened:**
- Parsed all 19 session JSONL files from `.claude/projects/`, extracted user prompts, message counts, tool call counts, and file sizes.
- Filtered out noise (exit sessions, internal-only sessions).
- User asked for a richer output: drop trivial sessions, expand each substantive session with follow-up instructions and a wrap-up, write to a markdown file.
- This file is the result.

---

## May 1 — Session 16: Event History Log & Dashboard Tab
**Session ID:** `current` · **PR:** #69

**Opening prompt:**
> "commit changes raise and merge pr."

**What happened:**
- Reviewed all pending changes across `internal/scheduler/`, `internal/web/`, and `internal/web/static/index.html`.
- Identified that `events.jsonl` and platform-specific binaries (`sentinel-macos-amd64`, `sentinel-macos-arm64`) were untracked and should be gitignored — updated `.gitignore` with `sentinel-*` and `events.jsonl` patterns.
- Split changes into two focused commits: (1) the event history feature (eventlog.go, scheduler integration, `/api/events` endpoint, History tab); (2) docs (CLAUDE.md session log convention + initial session-log.md).
- Raised PR #69 with full description of every changed file, the rationale for JSONL format, atomic prune, and retention window. Gotchas section called out `--local` mode behaviour and prune reset-on-restart.
- Merged and cleaned up the branch.

**Wrap-up:** Block/unblock event history log shipped — persists 30 days of transitions, pruned daily, surfaced in the web dashboard History tab behind the existing auth middleware.

---

## May 1 — Session 17: Pomodoro Focus Session with Lock-Down Mode
**Session ID:** `streamed-popcorn` · **PR:** #71 · **Release:** v0.1.11

**Opening prompt:**
> "lets work on issues 45 and 57 together. lock down mode only activated with pomodoro timer. another indepedent locked session mode is not needed I think. what do you say? show me a plan."

**What happened:**
- User decided to merge issues #45 (locked sessions / commitment mode) and #57 (Pomodoro timer) into a single feature: starting a Pomodoro work session is the only way to activate lock-down mode — no standalone lock button.
- Added `PomodoroSession` struct and four config methods (`IsLockedByPomodoro`, `StartPomodoro`, `AdvancePomodoroPhase`, `ClearPomodoro`) to `internal/config/config.go`.
- Extended `EvaluateRulesAtTime` in `internal/scheduler/scheduler.go` to force all `IsActive` rules on during a work phase (stricter than normal scheduling). Added phase-transition logic in the `evaluateRules` tick (work→break, break→clear) with macOS notifications via the existing `scriptExecutor` interface.
- Added `POST /api/pomodoro/start`, `DELETE /api/pomodoro` endpoints to `internal/web/server.go`; 423 Locked guards on `POST /api/pause` and `POST /api/config/update` during work phase; Pomodoro state exposed in `GET /api/status`.
- Dashboard Status tab gets a Pomodoro panel (start controls → live countdown + lock indicator during work phase → stop button during break). Manage tab shows a lock overlay instead of pause buttons during work phase.
- User asked about the blocked-domains list during a focus session — added a contextual note "🔴 Focus session active — all scheduled domains are forced on" below the list.
- Fixed a UI bug: after break expiry the "Stop session" button would persist until the scheduler tick cleared the session (~60s). Added a client-side check to detect an expired break phase from the status response and revert to start controls immediately.
- 19 new tests across `config`, `scheduler`, and `web` packages; all green.

**Wrap-up:** Pomodoro focus session shipped as a unified commitment tool — work phase locks the API and forces all active rules on, break phase restores normal scheduling, session self-clears on break expiry. Released as v0.1.11.

---

## May 2 — Session 18: AdGuard Home Comparison & Issue #53 DNS-TTL Quota Design
**Session ID:** `i-ticklish-falcon` · **PR:** #73

**Opening prompt:**
> "analyze issue 53. I am thinking this is not great because the user may not actually be on reddit the entire time. so this is not a reflection of site usage. it will be only useful if we can actually track usage."

**What happened:**
- Explored three approaches to quota tracking: event-log (unblocked time), DNS query counting (visits), and AppleScript tab polling (tab open time). Identified DNS-TTL timing as the cleanest option — no new permissions, cross-browser, passive idle tabs stop generating DNS traffic.
- Confirmed macOS Screen Time has the same gap: only tracks per-site usage in Safari; Chrome/Firefox/Arc appear as app-level time only. API is locked to MDM entitlements.
- Researched AdGuard Home scheduling in depth. Confirmed three hard limitations all tracked as their open issues: single time slot per day (#7253), one shared schedule for all services (#7146), predefined catalog only — no custom domain groups (#1692). All are capabilities Sentinel already has today.
- Documented AdGuard Home comparison in README FAQ section, docs/index.html (new accordion item with comparison table, updated "Schedule-based, per group" feature card), and CLAUDE.md documentation reminder. Shipped as PR #73.
- Researched background tab DNS behavior for issue #53 plan: Chrome caps its internal DNS cache at 60 seconds and SPAs like Reddit/YouTube maintain WebSocket/polling connections that bypass background timer throttling — so background tabs of social media sites WILL consume quota. Documented as a known limitation.
- Revised issue #53 implementation plan to DNS-TTL bucket counting (5-minute windows): `usagelog.go` at proxy layer, `groupLookup map[string]string` passed from scheduler to proxy, quota check in `EvaluateRulesAtTime`, 60-day retention, new Usage tab in dashboard. Requires dns or strict mode; hosts mode warning in UI.
- Posted detailed implementation plan as a comment on issue #53.

**Wrap-up:** AdGuard Home comparison landed in docs (PR #73); issue #53 has a complete DNS-TTL quota implementation plan with background tab behavior confirmed and all limitations documented.

---

*Generated from Claude Code session history on 2026-05-01.*

---

## May 2 — Session 19: Issue #53 Implementation — Daily DNS Quota Tracking
**Session ID:** `i-correct-moth` · **PR:** #74

**Opening prompt:**
> "pick up the plan documented in issue 53 for implementation. work in a feature branch and ensure you work in incremental steps so that if we run out of tokens in the middle of implementation, we can pick up again."

**What happened:**
- Verified issue #44 (block event log) was already closed and the prerequisite was satisfied.
- Fetched the full implementation plan from the last comment on issue #53 (DNS-TTL 5-minute bucket approach).
- Implemented in 7 atomic commits on `feat/issue-53-daily-quota`:
  1. `config`: added `DailyQuotaMinutes int` field to `Rule` struct (zero-value = no quota, backward-compatible).
  2. `proxy/usagelog.go`: new file — `UsageEvent` struct, append/read/prune functions, `ComputeGroupUsageMinutes` using 5-min bucket deduplication.
  3. `proxy/dns.go`: added `groupLookup map[string]string` package var and `UpdateGroupLookup` function; `handleDNSRequest` logs non-blocked queries for group domains.
  4. `scheduler`: added `BuildGroupLookup`, changed `EvaluateRulesAtTime` to accept `quotaUsage map[string]int`, added quota enforcement pass, midnight prune of `usage.jsonl` at 60 days, updated all test call sites to pass `nil`.
  5. `web/server.go`: `StatusHandler` returns `quotas[]` array (group, quota_minutes, used_minutes, quota_exceeded, mode_compatible); new `UsageHandler` for `GET /api/usage?range=today|7d|30d|60d`.
  6. `web/static/index.html`: quota progress bars in Status tab (green→amber→red), warning badge for hosts mode, new Usage tab with range selector and per-group/per-domain tables.
  7. Docs: README field reference + API table, DESIGN quota-tracking subsection with known limitations, TROUBLESHOOTING two new entries, landing page feature card + FAQ entry.

**Wrap-up:** Full DNS-TTL quota implementation shipped as PR #74. All 7 packages build and all tests pass.

---

## May 2 — Session 20: Strict Mode pf Firewall — End-to-End Fixes
**Session ID:** `45e8b2f6` · **PR:** #76 (fix/strict-mode-pf)

**Opening prompt:**
> "debugging help needed. DNS mode domains are not blocked... nslookup discord.com shows getting recursion not available from 127.0.0.1, trying next server..."

**What happened:**

This session started with a DNS-mode bug and expanded into a multi-turn deep dive on why strict mode pf blocking wasn't working end-to-end. What looked like a single problem turned out to be five independent bugs stacked on top of each other, each masking the next.

**Bug 1 — DNS proxy not setting the RA bit (root cause of initial report)**

`nslookup` was saying `Got recursion not available from 127.0.0.1, trying next server` and falling through to the backup DNS, so blocked sites were still resolving. The `GetDNSResponse` function in `internal/proxy/dns.go` was building reply messages without setting `m.RecursionAvailable = true`. Any stub resolver that checks the RA bit treats RA=0 as "server doesn't recurse" and queries the next configured server. Fix: one line. This was shipped in a separate PR (#75, v0.1.12) immediately.

**Bug 2 — AppleScript log noise**

While reviewing logs to diagnose strict mode, the user noticed `MacOSScriptExecutor.LogScript` was logging the full AppleScript text every scheduler tick (every 60 seconds), and `ExecuteScript` was logging "application isn't running" (-600) errors every minute because Safari wasn't open. Both are expected conditions that should be silent. Fixed: `LogScript` changed to no-op, `-600` errors suppressed in `ExecuteScript`.

**Bug 3 — `atomicWrite` using read-only root as temp directory**

Logs showed: `pf: update pf.conf: open /.pf-tmp-3309907590: read-only file system`. The `atomicWrite` helper in `internal/pf/pf.go` had `dir := "/"` hardcoded as the temp file location. macOS uses a sealed read-only root volume; temp files cannot be created there. Fix: changed to `dir := filepath.Dir(path)` so the temp file lives in the same directory as the target file (`/etc/`), which is writable.

**Bug 4 — `0.0.0.0` / `::` poisoning the pf table**

After fixing atomicWrite, pf activated but logs showed `pf: no IPs resolved for domains`. On closer inspection, the anchor file was being written with `0.0.0.0` and `::` — Sentinel's own blocked-domain responses. `ResolveDomainIPs` was querying `primaryDNS`, which is `127.0.0.1:53` (Sentinel's proxy). When a domain is actively blocked, the proxy returns `0.0.0.0`/`::`. The old code added those addresses to the pf table, then filtered them as a separate step; the `net.LookupHost` fallback also went through the system resolver (still pointing at `127.0.0.1`), producing the same poisoned response. Fix: filter unspecified addresses (`rr.A.IsUnspecified()`, `rr.AAAA.IsUnspecified()`) directly in `ResolveDomainIPs`, remove the `net.LookupHost` fallback entirely, and add a `backupDNS string` parameter to `ActivateBlock` so strict mode can re-resolve using `1.1.1.1` or the configured backup DNS when primary returns nothing.

**Bug 5 — IPv6 `primary_dns` with malformed host:port**

After adding the `backupDNS` fallback, logs showed `pf: no IPs resolved` again. Checking the config, `primary_dns` was `2001:558:feed::1:53` — an ISP-assigned IPv6 DNS server. Go's `net.Dial` requires IPv6 addresses to be bracketed: `[2001:558:feed::1]:53`. The unbracketed form parses ambiguously (the last `:53` could be part of the IPv6 address). `detectSystemDNS()` in `internal/enforcer/dns.go` was formatting all detected DNS addresses as `ip + ":53"`, which produces a malformed address for IPv6. Fix: added a `hostPort(host, port string) string` helper that wraps IPv6 in brackets, used for all DNS address formatting in that file.

**Bug 6 — pf anchor syntax error (table declarations not allowed in anchor files)**

After real IPs were finally being resolved, the anchor file was still failing to load: `/etc/pf.anchors/sentinel:1: syntax error`. The original `GenerateAnchorContent` used `table <blocked_ips> persist { ... }` syntax, which modern macOS pfctl accepts in `/etc/pf.conf` but rejects in anchor files loaded via `pfctl -a anchor -f file`. The workaround: inline the IPs directly in the block rules, split by address family (inet / inet6). macOS pfctl then automatically promotes the inline lists to internal `__automatic_*` tables. Fix: rewrote `GenerateAnchorContent` to produce `block drop out quick inet proto {tcp udp} from any to { ip1 ip2 }` and `block drop out quick inet6 proto {tcp udp} from any to { ip6_1 ip6_2 }`. Also rewrote `DeactivateBlock` — the old implementation flushed a named table that no longer exists; new version writes `# no IPs to block` to the anchor file and reloads the anchor.

**Bug 7 — CDN IP rotation: IPs go stale between Activate calls**

After all six bugs were fixed, `discord.com` was blocked correctly but `facebook.com` was not, even though `nslookup facebook.com` returned `0.0.0.0`. Investigation: `pfctl -a sentinel -t __automatic_*_0 -T show` showed the IPs from when the block was activated; `dig facebook.com @1.1.1.1` returned a different IP (`57.144.22.1`) not in the table. Facebook serves from a large CDN and rotates IPs constantly. The comment in `StrictEnforcer.Activate` said "re-resolve ALL currently blocked domains on every activation", but `Activate` is only called by the scheduler when the *set of blocked domains changes* — during a steady-state block window, `Activate` is never called again, so IPs go stale indefinitely. Fix: added a `Refresh()` method to the `Enforcer` interface. `StrictEnforcer.Refresh()` re-resolves all currently blocked domains and reloads the pf anchor. `DNSEnforcer.Refresh()` and `HostsEnforcer.Refresh()` are no-ops. The scheduler calls `activeEnforcer.Refresh()` every tick when blocks are active and no domains changed.

**Verification methodology documented**

Added a comprehensive strict mode diagnostics section to `TROUBLESHOOTING.md`, including:
- The correct `pfctl` command sequence to verify each layer (pf enabled → anchor registered → rules loaded → IPs in `__automatic_*` tables)
- How to correlate the current real IP of a CDN site with what's in the tables
- Explanation of why `pfctl -a sentinel -t blocked_ips -T show` (the old command) no longer works
- The chicken-and-egg issue with `primaryDNS` pointing at the proxy and why `backupDNS` is required
- Known limitations: ≤60 s CDN gap, pre-existing connections, IPv6 must be covered

**Wrap-up:** Six root-cause bugs in strict mode pf blocking fixed across `internal/pf/pf.go`, `internal/enforcer/` (all three backends), and `internal/scheduler/scheduler.go`. Strict mode now correctly resolves, loads, and periodically refreshes IP-level firewall rules. Testing confirmed discord.com blocked; facebook CDN rotation fix in place but requires further testing to confirm. Raised as PR #76.

---

## May 3 — CDN coverage, DoH firewall bypass fix, and strict-mode self-heal → v0.1.14
**Session ID:** `a2f1c8d3`

**Opening prompt:**
> "merge both open PRs 78 and 80. create a new release next version and ensure to document the release notes"

**What happened:**

PR #78 (`fix(strict): comprehensive CDN/asset coverage + DoH opt-in group`) and PR #80 (`fix(strict): port-restricted pf rules for DoH/DoT + self-heal on mode downgrade`) were both open and ready. PR #78 targeted `main`; PR #80 stacked on #78's branch.

Merged PR #78 first. GitHub auto-closed PR #80 when the base branch was deleted on merge — a known GitHub behavior for stacked PRs. Reopening and retargeting a closed PR via the API is blocked by GitHub, so a replacement PR (#83) was created for the same `fix/doh-pf-port-blocking` branch, now targeting `main` directly. CI passed, PR #83 was merged.

Together the two PRs delivered:
- **CDN/asset domain expansion** for social, videos, and games groups — apex-only lists grew to cover the CDN domains responsible for the bulk of page bytes (fbcdn.net, tiktokcdn.com, googlevideo.com, ytimg.com, steamcommunity.com, etc.)
- **Multi-resolver IP union** — `ActivateBlock` queries both primary and backup DNS and unions the results, widening pf coverage for geo-distributed CDN edges
- **`_doh` opt-in group** of 14 common DoH/DoT endpoints added to the default config
- **Port-restricted two-section pf anchor** — section 1 all-port blocks for regular site IPs; section 2 TCP/443 + TCP+UDP/853 blocks for DoH/DoT IPs, leaving UDP/53 open so the daemon's own backup_dns keeps working
- **`_doh` flipped to default-active** — with port-restricted pf rules in place, the risk of breaking backup_dns is gone
- **Self-healing pf cleanup on mode downgrade** — `enforcer.New` calls `pf.RemoveAnchorIfPresent` for non-strict modes, catching stale anchor state left behind by a crash or SIGKILL
- 4 new pf unit tests covering the mixed-anchor generation logic

Release `v0.1.14` created with comprehensive release notes. The release workflow triggered automatically and will attach macOS arm64, macOS amd64, and Windows amd64 binaries plus `install.sh`.

**Wrap-up:** PRs #78 and #83 merged into main, release v0.1.14 live at https://github.com/vsangava/sentinel/releases/tag/v0.1.14. Chrome's DoH IP-upgrade bypass is now closed in strict mode.

---

## May 3 — Per-tick browser tab closer → v0.1.15
**Session ID:** `streamed-firefly` · **PR:** #84 · **Release:** v0.1.15

**Opening prompt:**
> "review issue 81 and tell me what you think and if makes sense. You can read issue 77 for more context of all the problem we had to get the domain blocking right in strict mode."

**What happened:**

Issue #81 proposed running the AppleScript tab-closer on every scheduler tick instead of only at the unblocked → blocked transition. The premise: even after v0.1.14's CDN coverage and DoH port-restricted rules, strict mode still has fundamental gaps (Safari iCloud Private Relay, certain DoH-upgraded paths, geo-anycast IP mismatches), and the tab-closer is the only mode-agnostic, OS-level enforcement we have. The transition-only trigger also missed a real bug case: a tab opened *during* an active block window — even via a perfectly normal mid-window page load — was never closed because `activeBlocks` already contained the domain, so there was no transition to hook on.

Reviewed and recommended shipping. The user confirmed with four refinements: (1) replace the transition trigger entirely (no double-running), (2) improve the notification copy, (3) filter the `_doh` group out of the browser probe (DoH endpoints aren't sites users visit with browsers), (4) make close + notify atomic in one osascript invocation.

Implementation in `internal/scheduler/scheduler.go`:
- New `runPerTickCloseTabs(blocked, cfg, probe)` invoked every tick after the enforcer block, gated by an injectable `browserTabProbe` (defaults to `getOpenBrowserDomains`).
- New `browserTargetableDomains` filters out the `_doh` group via `cfg.ResolveGroup(enforcer.DohGroupName)`. `dohGroupName` was exported so the scheduler could reuse the constant from `internal/enforcer/strict.go` rather than re-declare the magic string.
- `GenerateCloseTabsScript` now tracks a `closedCount` accumulator across all four browser blocks (Chrome / Safari / Arc / Brave) and emits a single trailing `display notification` ("Closed N tab(s) on facebook.com, tiktok.com" — or "across N sites" for >3 domains) when count > 0. One osascript invocation does both close and notify.
- 9 new tests covering the script changes, the DoH filter, and the per-tick driver including a regression test for the issue #81 case (domain already in `activeBlocks`, no transition, but tab is open → close fires).

The user asked mid-implementation whether the script handles incognito/private windows. Investigated each browser's AppleScript dictionary on disk:
- **Chrome** — `scripting.sdef` defines `window.mode` with values `"normal"` / `"incognito"`; the `windows` enumeration includes incognito windows. ✅ Covered.
- **Brave** — Chromium-based, inherits Chrome's scripting model. ✅ Almost certainly covered.
- **Arc** — Chromium-based with a custom window architecture (Spaces, Little Arc); not installed locally to dump the sdef. ✅ Most likely covered.
- **Safari** — `Safari.sdef` deliberately omits any private-browsing surface; private windows are hidden from AppleScript by Apple's design as a privacy guarantee. ❌ Cannot be closed via automation. The architectural fix is an MDM `SafariAllowPrivateBrowsing = false` payload.

Documented the Safari limitation as a coverage table in `TROUBLESHOOTING.md §4 macOS AppleScript path`. Updated `DESIGN.md` (tick-loop pseudocode + AppleScript section), `README.md` (feature blurb), and `docs/index.html` (landing-page card) to reflect per-tick cadence and the bypass scenarios it addresses.

CI failed once on the initial push with a parallel-test flake in `internal/testcli` (testcli and web both mutate `./config.json` with `UseLocalConfig = true`; race produces `unexpected end of JSON input`). Reproduced as a known issue independent of this change — a rerun passed cleanly. Worth flagging as a separate cleanup later: serialise the test packages that touch `./config.json`, or stop using the live config file for tests.

**Wrap-up:** PR #84 merged into main, release v0.1.15 live at https://github.com/vsangava/sentinel/releases/tag/v0.1.15. The macOS tab closer is now the genuine OS-level backstop for whatever bypasses get past DNS / pf — at least for any tab that isn't in Safari Private Browsing.

---

## May 3 — Per-tick tab closer fixes (round 2) → v0.1.16
**Session ID:** `streamed-firefly` (continued) · **PR:** #86 · **Release:** v0.1.16

**Opening prompt:**
> "I currently have both facebook.com and tiktok.com open in browser window. strict mode is on and running the latest version. why are the tabs not closing? Is there a bug in the applescript? How do we check?"

**What happened:**

v0.1.15 shipped the per-tick scaffolding and *all the tests passed*, but in production every tick failed silently end to end. The fix took **three iterations**, each of which unmasked the next layer of bug. This entry captures all three because the iteration sequence is the lesson — the tests we shipped in v0.1.15 covered the per-tick driver and the script generator independently, but never exercised the actual osascript fork against real browsers, which is where every one of these bugs lived.

### Iteration 1 — probe was running as root

Daemon logs showed every tick:
```
2026/05/03 12:10:44 Error checking open browser domains: exit status 1
```

Traced to `getOpenBrowserDomains` in `internal/scheduler/scheduler.go` calling `exec.Command("osascript", "-e", script).Output()` directly. The daemon runs as root under launchd; root has no GUI session attached to Chrome / Safari / Arc / Brave, so AppleScript exited 1 every time. The new per-tick close path was gated on this probe → probe always returned empty → close never fired.

`runAsMacUser` (already in the codebase, used by the *close* script) handles this correctly via `su - <console user> -c osascript ...` when running as root. But it returns only error/no-error, no stdout. The probe needs stdout to parse the matched-domain list.

**Fix:** new `runOsaScriptCapture(script string) (string, error)` helper mirroring `runAsMacUser`'s root → console-user shell-out and capturing stdout. Uses a separate tmpfile (`/tmp/df_probe.scpt`) so probe and close calls within a single tick don't clobber each other. Side benefit: the 3-minute pre-block warning, which uses the same probe, has been silently broken under launchd for as long as the warning has existed — fixed as a side effect.

### Iteration 2 — `is running` returned stale true, Safari errored with -600

After iteration 1, the probe was reaching the browsers, but every tick logged:
```
osascript exit 1: /tmp/df_probe.scpt:990:1320: execution error: Safari got an error: Application isn't running. (-600)
```

Even when Safari was closed. `if application "Safari" is running` returns a stale `true` via Launch Services when Safari has been launched in the user's session but isn't currently running (process gone, registry not cleared). The inner `tell application "Safari"` then errored with -600. With four browser blocks chained, one bad browser killed the whole probe.

Reproduced empirically with a minimal AppleScript: without `try`, Safari's stale-true case exits 1; with `try ... end try` wrap, exits 0. Applied the wrap to **each browser block** in both the probe and close scripts.

### Iteration 3a — full reverse iteration (broken: only worked when all four browsers installed)

After iterations 1 and 2, with two windows × three matching tabs each, only 1–2 tabs closed per tick — and inconsistently. Diagnosis: the close phase iterated `tabsToClose` forward; each entry is a specifier like `tab 2 of window 1` resolved at close time. After `close tab 1 of window 1`, `tab 2` now points to what was tab 3. Forward iteration over a mutating tab list closes the wrong tabs and skips the rest.

First attempt: rewrite as full reverse iteration with explicit indexed access (`tab tIdx of window wIdx`). All tests passed locally. Then in production:
```
osascript exit 1: /tmp/df_script.scpt:1668:1672: script error: Expected end of line but found identifier. (-2741)
```

Whole-script compile failure — Chrome and Safari blocks didn't run either. Empirically isolated: `tab tIdx of window wIdx` requires the app's AppleScript dictionary to be loaded at compile time. The user's machine doesn't have Arc or Brave installed (common — most folks have one Chromium browser, not three), so the parser couldn't resolve the indexed-tab syntax. The previous forward-iter syntax (`repeat with t in tabs of w` / `URL of t`) compiles fine without the dictionary because it uses generic terminology.

### Iteration 3b — collected list, closed in reverse (final fix)

Keep the dictionary-independent forward collection, walk the collected list in reverse at close time:
```applescript
set tabsToClose to {}
repeat with w in windows
    repeat with t in tabs of w
        ... if match: set end of tabsToClose to t
    end repeat
end repeat
set toCloseCount to count of tabsToClose
repeat with i from toCloseCount to 1 by -1
    try
        close item i of tabsToClose
        set closedCount to closedCount + 1
    end try
end repeat
```

`tabs of w` returns specifiers in index-ascending order, so the collected list is index-ascending per window. Walking it in reverse means the highest-index tab closes first, and lower-index specifiers in the list don't shift out from under us. Per-close `try` handles the case where closing the last matching tab in a window also closes the window (subsequent references would error — caught and skipped).

### Tests

Three new regression tests, each guarding the trap that bit us:

- `TestRunOsaScriptCapture_NonDarwinIsNoOp` — guards the no-fork path on Linux CI.
- `TestGenerateCloseTabsScript_WrapsEachBrowserInTryBlock` — asserts ≥4 `try`/`end try` pairs.
- `TestGenerateCloseTabsScript_ClosesCollectedTabsInReverse` — asserts the reverse close-loop syntax, AND explicitly **forbids** both the buggy forward iteration AND the dictionary-dependent `tab tIdx of window wIdx` form. Catches both regression directions: someone reverting to forward iteration, or someone "cleaning up" to indexed access that breaks on machines without all four browsers installed.

### Lessons

1. **Unit tests of the per-tick driver and the script generator are not the same as testing the actual osascript fork against real browsers.** All three bugs lived in the gap. Worth thinking about how to add an integration smoke test that runs the generated script through `osacompile` (compile-only, no exec) at minimum — would have caught iteration 3a. Cheap to add; flagged for a follow-up.
2. **AppleScript's behaviour depends on which app dictionaries are installed locally.** "It compiles on my machine" means nothing if the script targets browsers the user doesn't have. Generic-terminology syntax (`repeat with t in tabs of w`) is the dictionary-independent path; explicit indexed access (`tab N of window M`) requires the dictionary.
3. **`is running` is unreliable.** Launch Services can return stale `true`. `try ... end try` around each browser block is the cheapest defensive pattern; switching to System Events process probe (`tell application "System Events" to (exists process "X")`) is the more authoritative alternative but adds an Automation permission requirement.
4. **`runAsMacUser` was already in the codebase**, but the probe path skipped it. The 3-minute warning has been broken under launchd as long as it's existed — nobody noticed because warnings are subtle. Worth auditing for other osascript call sites that should be going through the helper.

**Wrap-up:** PR #86 merged into main, release v0.1.16 live at https://github.com/vsangava/sentinel/releases/tag/v0.1.16. User confirmed live behaviour: tabs in two windows × three tabs each all close correctly within 60 s. The per-tick close path is now genuinely working — three iterations later than v0.1.15 implied.

---

## May 4 — `setup`/`clean` idempotency + test config isolation → v0.1.17
**Session ID:** `cda47b91` · **PRs:** #87, #88 · **Release:** v0.1.17

**Opening prompt:**
> "fix issues 79 and 85."

**What happened:**

Two unrelated bugs cleaned up in one session.

### Issue #85 — `setup` wedged in a self-contradictory loop

`sudo sentinel setup` aborted with `Sentinel is already installed at /usr/local/bin/sentinel. Run 'sudo sentinel clean' to remove it first` even when the user *had* run `clean`. Looking at `cmd/app/main.go:runClean`, the cleanup pipeline removed the launchd plist, the config dir, /etc/hosts entries, the pf anchor — but never the binary at `/usr/local/bin/sentinel`. Setup's existence guard then refused to proceed. Users were stuck unable to upgrade or recover without a manual `sudo rm`.

Initial fix added `cleanup.RemoveInstalledBinary()` as step 8 of the clean pipeline. Path captured in a package-private var so unit tests can substitute a tempdir. Before claiming done I wanted to convince myself the unlink was safe even when the cleaning process is itself `/usr/local/bin/sentinel` — wrote a tiny standalone Go program that `os.Remove`'d its own executable while running, confirmed the dirent went immediately, the kernel kept the inode alive for the running image, and the process exited cleanly.

**User pushback (the productive kind):** *"you tested a regular executable, but ours is a service installed. both are not same.. can you confirm?"* Fair point — the standalone test didn't exercise the launchd-managed daemon scenario. Wrote a more realistic test: built a tiny Go daemon, installed it as a real LaunchAgent with `KeepAlive=true`, watched it tick under `launchctl list`, then ran the cleaner sequence — `launchctl bootout` → plist removal → unlink the running binary — and confirmed launchd did NOT respawn 2 s later. The order of operations matters: by the time we unlink, the plist is gone, so `KeepAlive` has nothing to act on.

Then the user came back with: *"sudo sentinel clean && sudo sentinel setup failing with ... already installed"*. Checked `/usr/local/bin/sentinel` — dated April 30, predating today's fix. Of course: the *installed* binary still had the old `clean` code, so `clean` left the binary in place and the next `setup` (also old code) tripped the guard. Chicken-and-egg. The fix was correct but the user couldn't bootstrap into it.

Resolved with a second commit: made `setup` idempotent. Removed the "already installed" guard entirely. Setup now best-effort stops + uninstalls any existing service registration (so kardianos's install step doesn't trip on a duplicate plist), unlinks the old binary, writes the new one, and re-registers + starts. Re-running `setup` over an existing install just produces a working install — `clean` is no longer load-bearing for upgrades. Even users on v0.1.16 or earlier can now run `sudo ./sentinel setup` straight onto an existing install with the new binary; no manual `rm`, no `clean` required.

### Issue #79 — tests reformatting the checked-in config.json

The `testcli` and `web` test packages set `config.UseLocalConfig = true` and chdir'd up to the module root so `LoadConfig`/`SaveConfig` would hit `./config.json`. Two failure modes: (1) handlers under test (Pomodoro, Pause) call `config.SaveConfig`, which re-marshals with `json.MarshalIndent` — Go alphabetizes map keys, so `Monday → Sunday` becomes `Friday → Wednesday`, and the inline `[{...}]` arrays in the source file get exploded into multi-line objects on every test run. (2) Both packages wrote to the same `./config.json`, racing under `go test ./...`'s default parallel execution.

Fixed with `config.ConfigDirOverride` — a string that takes precedence over `UseLocalConfig` and the OS-specific defaults when non-empty. Each `TestMain` mints its own tempdir via `os.MkdirTemp` and points the override at it. `LoadConfig` falls back to writing the embedded `default_config.json` when the tempdir is empty, so tests still get a valid config. Verified with `go test -count=3 -p 8` that `git diff config.json` stays empty across repeated parallel runs.

### Lessons

1. **"Self-deletion works on macOS" generalises across process types.** Whether the running binary is a standalone CLI or a launchd-managed daemon, `unlink(2)` is filesystem-level and doesn't care. What changes between scenarios is the *respawn risk*, not the unlink semantics — and the order of operations (stop daemon → uninstall plist → unlink) is what controls respawn risk. The user was right to push back, and the more realistic test made the answer more durable.
2. **Idempotency beats guard checks.** The "already installed" guard in `setup` was defensive but counterproductive: it punted to `clean`, which (in old builds) didn't actually remove the binary. Idempotent `setup` removes the dependency on `clean` having particular behaviour. Same lesson would apply to any other "is the system already in state X?" check that errors out instead of converging — convergence is almost always a better default.
3. **Tests touching shared, checked-in files leak**. `UseLocalConfig` was a quick way to keep tests off the system config dir, but it just shifted the problem to a different shared file. Per-test or per-package tempdirs are the actually-isolated answer; an explicit override variable is enough to retrofit it without a big test-rewrite.

**Wrap-up:** PR #87 (issue #85, two commits: `clean` removes binary + `setup` idempotency) and PR #88 (issue #79, test isolation) merged into main. Release v0.1.17 live at https://github.com/vsangava/sentinel/releases/tag/v0.1.17. The release workflow's first run hit a transient `proxy.golang.org` networking failure on the macos-latest runner; re-running succeeded in 1m20s.

---

## May 4 — Sleep/wake behaviour FAQ (issue #72)
**Session ID:** `zazzy-starfish` · **PR:** #89

**Opening prompt:**
> "work on issue 72."

**What happened:**

Issue #72 was an explore-style question: *"Does Sentinel prevent a laptop from sleeping due to this constant 1 min timer?"* Investigated before assuming the answer. Findings:

- Only ticker in the repo is `time.NewTicker(1 * time.Minute)` in `scheduler.go:436` — a regular Go ticker is a goroutine sleeping on a channel, not a power assertion.
- Zero hits across the repo for `IOPMAssertion`, `caffeinate`, `ProcessType`, or any other power-management API.
- launchd plist generated by `kardianos/service` from `cmd/app/main.go` sets only `Name`, `DisplayName`, `Description` — no `ProcessType: Interactive`, no `KeepAlive`.
- DNS and web servers bind to `127.0.0.1` only — passive listeners that wake to handle inbound packets but never initiate activity.

Conclusion: the daemon does not prevent sleep. macOS will sleep on its normal idle schedule; the ticker resumes when the system wakes (with up to ~60 s before the next rule evaluation, which the user may or may not care about — out of scope here).

Asked the user to confirm what deliverable they wanted (docs vs. code change vs. empirical pmset evidence). They picked **docs only**. Added a single FAQ entry to `README.md` and `docs/index.html` answering the question in plain language and pointing to `pmset -g assertions` as a self-verify command. PR #89 was a 2-file, 8-line change.

**Lesson:** The temptation on an `explore:` issue is to skip the investigation and write a long doc. Better order: investigate first, scope the deliverable to the smallest thing that actually answers the question, only then write. The investigation-first step also caught that the codebase has grown beyond what `CLAUDE.md` describes (`enforcer`, `pf`, `cleanup` packages weren't in the architecture overview) — worth keeping in mind when working from project docs alone.

**Wrap-up:** PR #89 merged into main, issue #72 closed. No release cut — pure docs change.

---

## May 5 — Comprehensive DoH coverage docs (issue #82)
**Session ID:** `docs-issue-82-doh` · **PR:** TBD

**Opening prompt:**
> "continue what you were doing working on issue 82 as you finished 57. pick up from where you left off."

**What happened:**

Continuation session. Issue #57 (Pomodoro / focus sessions) shipped via PR #90 in the previous turn; this session was the docs sweep for issue #82, which asks: *"with recent changes that handle browser-DoH bypass, our product is now more effective than AdGuard Home — make sure that fact is reflected in the user-facing docs, the FAQ, the troubleshooting guide, and CLAUDE.md."*

The underlying code work for DoH coverage was already merged in earlier PRs:
- PR #78 — comprehensive CDN/asset coverage + opt-in `_doh` group.
- PR #83 — port-restricted pf rules for the `_doh` group (TCP/443 + TCP+UDP/853) + strict-mode self-heal on mode downgrade.

So this PR is purely docs catching up to what the code already does.

### CLAUDE.md
Architecture overview was stale (matched the codebase pre-`enforcer`/`pf`/`cleanup` packages — flagged in the previous session as worth fixing). Rewrote the "What this project does" intro to lead with the three enforcement modes and what each does. Updated the data-flow steps to mention the per-tick `pf.Refresh()` (CDN IP rotation), the per-tick AppleScript browser closer, and the new `_doh`-aware behaviour. Added rows to the package table for `internal/enforcer`, `internal/pf`, and `internal/cleanup`. Noted that strict mode is macOS-only and the factory falls back to dns mode on other OSes.

### docs/index.html (landing page FAQ)
Added a new FAQ accordion item: *"What about browsers using DNS-over-HTTPS (DoH or 'Secure DNS')?"* — walks through the three cases (default install, hosts mode, strict mode) and explains why each is covered. Updated the strict-mode card description to call out the `_doh` group explicitly. Added two rows to the AdGuard comparison table: *"Survives browser DNS-over-HTTPS (Secure DNS)"* and *"Kernel-level IP blocking (pf firewall)"* — both ✓ for Sentinel, ✗ for AdGuard. Updated the AdGuard intro paragraph to note the DoH advantage.

### README.md
Mirrored the landing-page FAQ entry, and added the same two AdGuard comparison rows so the README table matches what's on the site. The README's "Configuration" section already had a deep `_doh` group write-up from earlier work, so no further edits there.

### TROUBLESHOOTING.md
Already had a comprehensive *"Browser DNS-over-HTTPS (DoH) bypass"* section (added in PR #83). Two additions this round:
1. **One-shot health check.** A copy-paste bash block under §4 → strict mode that runs every layer's check (service status, mode, sys DNS, dig, `pfctl -s info`, anchor presence, rule count, table dump, anchor file on disk, pf.conf injection, last-60s logs) in sequence. Lists the expected healthy output beneath the block so the reader can compare. Builds on what I'd been pasting into Claude Code while debugging strict-mode issues across PRs #78/#83/#86.
2. **`_doh` always-on detail in the DoH section.** Added a verification command (`sudo pfctl -a sentinel -s rules | grep -E 'port = (443|853)'`) and the expected rule-set layout so users on strict mode can confirm the `_doh` rules actually loaded.

### DESIGN.md
Anchor-file model section was the most stale — said "single `<blocked_ips>` table" with one rule, but the current code generates a two-section anchor. Replaced that with the actual two-section layout (regular IPs all-port + DoH IPs port-restricted on TCP/443 and TCP+UDP/853) and explained the rationale: regular blocked IPs get all-port drops because there's no legitimate traffic to those CDNs; DoH endpoints get port-restricted drops so UDP/53 plain DNS to the same IPs (1.1.1.1, etc.) stays open for `backup_dns`. Updated the `ActivateBlock` description to match the current `ActivateBlockMixed` signature (`domains`, `dohDomains`, `primaryDNS`, `backupDNS`) and noted the deliberately-asymmetric state-kill behaviour: only regular block IPs get `pfctl -k`'d, DoH IPs are left alone to preserve the daemon's own backup-DNS sessions.

### Lessons

1. **Docs PRs that follow a code PR are easy to defer and easy to skip.** PR #78 added the `_doh` group, PR #83 made it port-restricted and always-on, PR #86 reorganised tab-closer behaviour around DoH-bypassed tabs — but the user-facing FAQ never said the word "DoH" until this PR. Worth flagging in a future workflow: when shipping a feature whose value only makes sense when the user *understands* an obscure threat model (DoH bypass), the doc update is part of the feature, not optional.
2. **Docs drift from code shows up in the architecture sections first.** CLAUDE.md and DESIGN.md both still described the pre-multi-mode code layout. Worth a periodic `grep -L 'enforcer'` style audit against what actually exists in `internal/`.

**Wrap-up:** PR opened with five files updated (CLAUDE.md, README.md, docs/index.html, TROUBLESHOOTING.md, DESIGN.md). Pure docs change — no release cut.

---

## May 6 — Foreground-tab time tracking, opt-in (issue #92)
**Session ID:** `feat-issue-92-foreground` · **PR:** TBD

**Opening prompt:**
> "continue what you had planned to do on issue 92. your entire plan is added as a comment and I also added a latest comment to the issue with my response that you need to consider."

**What happened:**

Plan was already in #92 as a comment from the prior session: extend the per-tick AppleScript surface so the active browser tab's domain is recorded as a *separate* metric — `foreground_minutes` — alongside the DNS-bucket `used_minutes`. User's response on the comment crystallized the constraints:

1. Keep the two metrics *separate*; do not feed foreground time into `daily_quota_minutes`.
2. Browsers only for now — file a follow-up for non-browser apps (Slack, Discord, Xcode).
3. File a follow-up for Windows.
4. Behind a config flag.
5. Confirm and call out that foreground tracking works in `hosts` mode (where DNS-bucket tracking does not).
6. Privacy floor: only track domains in any configured group, excluding `_doh`.

That last point is the big one — without it the feature silently expands from "tracking what you opted into" to "tracking everything you browse." Codified in `trackedDomainSet()` (`scheduler/foreground.go`) which walks `cfg.Groups` and skips `enforcer.DohGroupName`.

### Implementation shape

- `config.Settings.EnableForegroundTracking bool` — opt-in, default false.
- `proxy.UsageEvent` gained `Kind` (`"" | "dns" | "foreground"`). Empty = legacy = DNS, so pre-feature `usage.jsonl` entries keep aggregating without migration.
- New `scheduler/foreground.go`: AppleScript probe returns `frontmost_app<TAB>active_url<TAB>idle_seconds`. Idle from `ioreg | awk` on `HIDIdleTime` (no entitlement). Per-browser URL access uses `active tab` for Chrome/Arc/Brave, `current tab` for Safari (Safari is the odd one out — important).
- Gating in `recordForegroundTick`: empty probe output is a clean no-op (non-darwin path), idle ≥ 60s suppresses the event, frontmost-app must be one of four supported browsers, URL must parse as `http`/`https` with a non-empty host (filters `chrome://newtab/`, `about:blank`), host (lowercased + `www.` stripped, subdomain-aware) must match a configured non-`_doh` domain.
- Aggregation: `proxy.ComputeGroupForegroundMinutes` uses **1-minute** buckets — naturally minute-granular because the scheduler ticks each minute and emits at most one event per tick. DNS aggregator now filters by `IsDNSKind()` so the two signals stay independent.
- `/api/usage` adds `foreground_minutes` to both group rows and domain rows; the dashboard adds a Foreground (min) column. The "no data" hint distinguishes DNS (needs dns/strict) vs foreground (needs the flag).

### Cross-mode confirmation (the user's point #5)

The probe lives in the scheduler tick, not in the enforcer — exactly the same call site as the existing per-tick close-tabs path. It writes to `usage.jsonl` directly via `proxy.AppendUsageEvent`. There is no DNS proxy in the path. Concretely:

| Mode | DNS-bucket `used_minutes` | Foreground `foreground_minutes` |
|------|---------------------------|----------------------------------|
| `hosts` | unavailable | works |
| `dns` | works | works |
| `strict` | works | works |

So foreground tracking is the *only* per-domain time signal in `hosts` mode — a meaningful change for users who run hosts mode for simplicity but still want to see where their time goes. Called out in DESIGN.md and the README field reference.

### Edge case: `extractHost` and `chrome://newtab/`

First attempt to extract host used `url.Parse(rawURL).Hostname()`. That returns `"newtab"` for `chrome://newtab/` because chrome:// is a valid URL scheme and "newtab" parses as the authority. Tracked-domain matching would then return "" (defense at the next layer), but it felt fragile — a user with a configured "newtab.example" *could* in principle get a false positive if a future browser ever exposed a similar URL scheme. Tightened `extractHost` to return "" for any non-http(s) scheme. Test had to be updated to match.

### Tests

Two new test files. `proxy/foreground_test.go` covers schema invariants — DNS aggregator must ignore foreground events, foreground aggregator must ignore DNS events, legacy empty-Kind events keep aggregating into `used_minutes`, `IsDNSKind` truth table. `scheduler/foreground_test.go` covers parser edge cases (malformed inputs, idle-non-integer), the tracked-domain set excluding `_doh`, the matcher's subdomain attribution and false-positive guards (`youtubex.com` must NOT match `youtube.com`), `extractHost` (lowercasing, www-stripping, scheme-gating), and the gating decisions in `recordForegroundTick` via a stub probe runner — happy path, idle skip, non-browser app, untracked domain, `_doh` excluded, `www.` stripping, internal browser URLs, and probe-error propagation.

### Follow-ups filed

- **#93** — foreground time tracking for non-browser apps (Slack, Discord, Xcode). Notes the data-shape change (no per-URL granularity), enumerates options, scopes out app-time quotas as a deliberately separate product call.
- **#94** — Windows-side exploration. Documents the Win32 / UI-Automation / extension trade-offs and aligns on reusing the same `Kind: "foreground"` event shape so the dashboard renders without code changes.

**Wrap-up:** Branch `feat/foreground-tab-tracking` opened with config flag, probe, parsing/matching helpers, scheduler wiring, usage-log schema bump, /api/usage and dashboard updates, and 18 new tests. Two follow-up issues filed (#93, #94). Docs updated across DESIGN.md, README.md, and docs/index.html.

## May 6 — Display running version on web dashboard (issue #96) → v0.1.19
**Session ID:** `feat-issue-96-version` · **PR:** [#98](https://github.com/vsangava/sentinel/pull/98)

**Opening prompt:**
> "implement issue 96. simple and straightforward so go ahead and implement, raise and merge pr, release next version."

**What happened:**

Issue #96 ("display current running version on web console") read trivially but exposed that the binary had no version string at all — `make` defaulted `VERSION ?= dev` for help text but never injected it into the binary, and there was no `internal/version` package to inject into.

Three pieces:

1. **`internal/version/version.go`** — single `var Version = "dev"` overridden via `-ldflags "-X github.com/vsangava/sentinel/internal/version.Version=..."`. Internal package keeps it from being importable outside the module (matches existing layering).
2. **`/api/version`** — public, no auth. Same rationale as `/api/config`: the dashboard needs to render the version pill *before* it has bootstrapped the auth token. Added route to both `StartWebServer` and `StartTestWebServer` and updated the auth-middleware comment that previously named only `/api/config` as the public exception.
3. **Dashboard pill** in the `app-header` — `margin-left: auto`, semi-transparent white, `font-variant-numeric: tabular-nums` so version digits don't reflow. Hidden by default; `loadVersion()` flips `hidden` after a successful fetch and silently no-ops on failure (so older daemons + newer dashboards just don't show the pill, no error).

**Wiring:**

- `Makefile` — added `LDFLAGS := -X github.com/vsangava/sentinel/internal/version.Version=$(VERSION)` and applied it to both `build` and `build-all` targets. `make build VERSION=v0.1.19` now bakes the value in.
- `.github/workflows/release.yml` — pulls `${{ github.ref_name }}` (the pushed tag) and passes it via `-ldflags`. Added `shell: bash` on the build step because the matrix includes a Windows runner where `pwsh` quoting of `-ldflags "-X foo=bar"` is unreliable. Git Bash is preinstalled on `windows-latest`, so `shell: bash` works uniformly.

**Verification gotcha:**

Tried `./sentinel --test-web` to curl `/api/version` locally and got 404 even though `/api/config` returned valid JSON. `lsof` showed nothing on 8040, but `pgrep -lf sentinel` revealed PID 55371 running as root — the *installed daemon* on this machine, predating my changes, holding port 8040. My test instance silently failed to bind (logged in `/tmp/sentinel.log`) and the curl was hitting the production daemon. Couldn't `kill` it without sudo, so verified two other ways: a unit test (`TestVersionHandler_ReturnsCurrentVersion` toggles `version.Version`, hits the handler, asserts the JSON body), and `strings ./sentinel | grep ^v0.1.19` to confirm the ldflags value made it into the binary.

**Tests:**

`TestVersionHandler_ReturnsCurrentVersion` saves the package var, sets a sentinel value, calls the handler, decodes the JSON, and restores via `defer`. Restoring matters because `version.Version` is module-global and the test runs alongside the rest of the web suite — leaving a stale value would leak into any later test that read it.

**Wrap-up:** PR [#98](https://github.com/vsangava/sentinel/pull/98) merged to `main`; tag `v0.1.19` pushed and the GitHub release workflow published `sentinel-macos-arm64`, `sentinel-macos-amd64`, `sentinel-windows-amd64.exe`, and `install.sh` to <https://github.com/vsangava/sentinel/releases/tag/v0.1.19>. Updated README's dashboard section to call out the version pill. Auto-update'd users will see their dashboard show `v0.1.19` once they restart the service.

## May 6 — Foreground-tracking fallout fixes → v0.1.20
**Session ID:** `fix-foreground-v0120` · **PRs:** [#100](https://github.com/vsangava/sentinel/pull/100), [#101](https://github.com/vsangava/sentinel/pull/101)

**Opening prompt:**
> "there is an issue with pr 95. the newly added config option `enable_foreground_tracking` is not working or it is not clear how to set it. I am getting this error when trying to update it from web Manage tab with error `invalid JSON: json: cannot unmarshal string into Go struct field Settings.settings.enable_foreground_tracking of type bool`."

**What happened:**

Two distinct bugs surfaced from #95 — one about the field's *visibility* and one about the probe never running at all on most macOS installs.

### Bug 1 — Config field type wasn't discoverable (#100)

The user had typed `"enable_foreground_tracking": "true"` (string) instead of `true` (bool) and hit a Go unmarshal error. Reading the dashboard's Manage tab, the inline help mentioned the field name but not its type or value, and none of the four example/default config blocks across the repo (`internal/config/default_config.json`, the README *Example config*, the DESIGN.md `Settings` struct snippet, and the dashboard's `EXAMPLE_CONFIG`) showed it. Same gap existed for `dns_failure_mode`, which had been added earlier — present in the bootstrap default-config file but missing from every other example surface. PR #100 added both fields to all four locations with documented defaults (`enable_foreground_tracking: false`, `dns_failure_mode: "open"`). No behaviour change — both have `omitempty` JSON tags and Go zero-values match the documented defaults — but spelling them out doubles as type documentation.

### Bug 2 — Probe AppleScript fails to compile when Arc/Brave aren't installed (#101)

After #100 landed, the user fixed their config and immediately hit a new failure in scheduler logs:

```
scheduler: foreground probe: osascript exit 1: /tmp/df_probe.scpt:792:795: script error: Expected end of line but found property. (-2741)
```

Reproduced locally. The probe script in `internal/scheduler/foreground.go` emits all four browsers' URL-fetch logic inline as `tell application "X" to set activeURL to URL of active tab of front window`. AppleScript compiles the whole script up front and resolves application terminology against each named app's scripting dictionary at compile time. `URL of active tab of front window` is not generic AppleScript — `active tab` and `front window` are properties defined by Chrome/Arc/Brave's dictionary; `current tab` is Safari's. With Arc and Brave not installed (the common case), the compiler can't resolve those property terms and emits **-2741: Expected end of line but found property**. The surrounding `try` cannot catch this — it's a parse error, not a runtime error. Net effect: the probe died every tick, `foreground_minutes` stayed at 0 regardless of `enable_foreground_tracking: true`. **The opt-in feature shipped in v0.1.18 was effectively broken on every machine that didn't have all four browsers installed.**

The existing close-tabs script (`scheduler.go:706` `getOpenBrowserDomains`) doesn't trip this because it only references `URL of t` where `t` is a local AppleScript variable — no app-specific dictionary needed at compile time. The probe needed `active tab` / `current tab` / `front window`, which are app-specific.

### Fix shape (PR #101)

Considered alternatives:
- **`using terms from application "Google Chrome"` borrowing.** Works only if Chrome is installed — fails on a Safari-only or Arc-only machine.
- **Block-form `tell application "X" ... end tell`.** Same compile-time terminology requirement as single-line form when inner content references app-specific properties (verified: -2741 still reproduces).
- **`if application "X" is running then` outer guard.** Doesn't gate compilation either — dictionary still has to load (verified).
- **Conditional script generation** (only emit branches for installed apps). More code than the win warrants.

Chosen: dispatch each browser's URL fetch through `do shell script "osascript -e '...'"`. Each nested `osascript` invocation runs in its own process and only compiles its own one-liner. Terminology resolution happens lazily — only the branch matching the actual frontmost app ever runs, and a not-installed app can never be the frontmost, so it can never be reached. The outer probe script no longer references any browser-specific dictionary terms, so it compiles cleanly on any macOS install regardless of which browsers are present.

Verified end-to-end by writing a one-shot test that calls `MacOSForegroundProbeGenerator.GenerateForegroundProbeScript()`, dumps the emitted bytes to `/tmp/df_probe_emitted.scpt`, and runs `osascript` on it — exits 0 with `Terminal\t\t<idle>` on a machine with only Chrome and Safari installed. Same machine reproduces -2741 against the pre-fix script.

### Cost

One extra `osascript` process per tick when a supported browser is frontmost. Negligible — the per-tick close path already shells out, this is one more, and only when the user is using a tracked browser.

**Wrap-up:** Both PRs ([#100](https://github.com/vsangava/sentinel/pull/100), [#101](https://github.com/vsangava/sentinel/pull/101)) merged to `main`; tag `v0.1.20` pushed and the release workflow published `sentinel-macos-arm64`, `sentinel-macos-amd64`, `sentinel-windows-amd64.exe`, and `install.sh` to <https://github.com/vsangava/sentinel/releases/tag/v0.1.20>. Foreground tracking is now actually functional in the released build for any macOS install with at least one of the four supported browsers, not just installs that happen to have all four. Users on v0.1.18/v0.1.19 with `enable_foreground_tracking: true` need to upgrade — the metric was producing zero data on most machines.

## May 7 — Hide DoH endpoints in dashboard blocked list (issue #97) → v0.1.21
**Session ID:** `feat-issue-97-hide-doh` · **PR:** [#103](https://github.com/vsangava/sentinel/pull/103)

**Opening prompt:**
> "did you not start issue 97?"

**What happened:**

Picked up #97 from the previous session's backlog: the dashboard's currently-blocked list dumped all ~11 `_doh` endpoints alongside user-scheduled domains, drowning out the rule-driven blocks the user actually cared about. Pure client-side fix in `internal/web/static/index.html` — `loadStatus()` now partitions `data.blocked_domains` against `liveConfig.groups._doh`, renders user sites in the existing red list, and tucks DoH endpoints into a collapsed `<details>` expander with summary text like *"11 DoH/DoT endpoints (infrastructure — click to expand)"*. Three empty-state branches: nothing-blocked, only-DoH-blocked (`✓ No scheduled domains currently blocked`), and the mixed case. No new API surface — both the blocked map and the DoH list are already exposed by `/api/status` and `/api/config`, and `loadStatus` is invoked through `loadConfig().then(loadStatus)` so `liveConfig` is always populated by the time the partition runs.

**Verification gotcha — the user's local config has no `_doh` group.**

After landing the change I told the user it was ready; they refreshed and reported "I tried and don't see a summary text". Spent a fair bit of time tracing why before realizing their working `./config.json` only has `games`, `videos`, `social` — no `_doh` group at all. With no DoH list to filter against, `dohBlocked` is empty and the expander correctly omits itself, so the dashboard looked unchanged. Issue #97 was filed against a config that did have `_doh` (the bundled `default_config.json` ships one with an always-on rule). The "fail-open when there's no `_doh`" behaviour is correct, but it meant my live test against the user's setup was a no-op — couldn't prove the partition worked just by curling against their config.

Killed the running daemon, mkdir'd `/tmp/df-test`, dropped a synthetic config there with `_doh` populated + an active rule, and ran `/tmp/sentinel-issue97 --local` from that directory so the daemon picked up the synthetic config without touching the repo's `config.json`. `/api/status` then returned 13 blocks (2 user + 11 DoH) and the dashboard rendered the expander as expected. Pointed the user at `127.0.0.1:8040` for a hard-refresh visual check — they confirmed it worked.

**UI tweak — chevron sized to match the red dot.**

User came back: "the chevron is too small / invisible. can you make it same size as the red dot that is shown in front of blocked domain names". Original implementation used a Unicode `▸` glyph at `font-size: 10px` — visible but thin and a bit anaemic next to the solid 7px red dot in the blocked list. Swapped it for a CSS-drawn triangle (the standard `border-top/bottom: 4px transparent + border-left: 7px solid #6c757d` recipe), `flex-shrink: 0` so it doesn't squeeze when the summary text wraps, and the existing `transform: rotate(90deg)` on `[open]` still works for the expansion animation. Visual weight now matches the dot.

**Wrap-up:** PR [#103](https://github.com/vsangava/sentinel/pull/103) merged to `main`; tag `v0.1.21` pushed and the release workflow published `sentinel-macos-arm64`, `sentinel-macos-amd64`, `sentinel-windows-amd64.exe`, and `install.sh` to <https://github.com/vsangava/sentinel/releases/tag/v0.1.21>. The dashboard's blocked list now reflects what users actually want to see — scheduled sites foregrounded, infrastructure tucked away one click deep.

## May 7 — Per-day usage-log rotation + 30-day retention (issue #105) → v0.1.22
**Session ID:** `feat-issue-105-rotate-usagelog` · **PR:** [#106](https://github.com/vsangava/sentinel/pull/106)

**Opening prompt:**
> "foreground tab tracking isn't working. can you generate a snippet of applescript that is actually used in the program to do this job, so I can run it directly and see why or what is not working."

**What happened:**

Started as a debugging request — user couldn't see `foreground_minutes` populating in the dashboard's Usage tab. Pulled the verbatim probe body out of `MacOSForegroundProbeGenerator.GenerateForegroundProbeScript` (`internal/scheduler/foreground.go:55-108`), wrote it to `/tmp/df_probe_manual.scpt`, and walked the user through running it directly. Their first run printed `Terminal\t\t<idle>` because Terminal was frontmost while Chrome sat behind it. Wrote a second variant (`/tmp/df_probe_delayed.scpt`) with `delay 5` at the top so they could switch to Chrome before sampling. That confirmed the AppleScript path is healthy and the metric was zero by design — they'd been keeping Terminal/editor focused while a YouTube tab played in the background, which the foreground tracker correctly does not attribute time to.

That conversation pivoted into a storage question: "writing one line per domain per min in usage.jsonl seems like it can take up a lot of space ... is it not efficient to summarize into a one line per domain per day in a per-day file?" Walked through the actual code (`internal/proxy/usagelog.go`) and grounded the discussion in what the read/write pattern *is* today: one ever-growing `usage.jsonl`, full-file decode every `/api/usage` render (even for "today"), full-file rewrite once a day to drop one day's worth of records, retention cap at 60 days. Foreground events are at most 1/min so they're not the volume driver — DNS is, with one event per blocked-group lookup.

Laid out three options:

1. **Per-day file rotation** (`usage-YYYY-MM-DD.jsonl`). Smallest change, biggest read-perf win for "today" (1/60th of the work), prune becomes `unlink`, no append-vs-prune race because today's file is never a prune candidate.
2. Two-tier (raw today + summarised history). Larger change, useful only if a single day's file ever gets uncomfortable.
3. Pure summary. Loses all intra-day fidelity forever.

User asked a sharp follow-up — "won't read take a very long time to read and parse 60 files?" Walked the cost: total decoded bytes are identical for a 60d query, plus ~5 ms of file-open overhead for 60 files on APFS, all dwarfed by JSON-decode throughput. For "today" / "7d" / "30d" views the new path is strictly faster (skips files outside the window at the directory layer). Confirmed option 1 is a strict improvement, never a regression. They greenlit option 1 and asked to fold in a retention cut from 60 → 30 days at the same time.

Filed [#105](https://github.com/vsangava/sentinel/issues/105) with the design (path helper keyed off event date, append/read/prune semantics, one-pass legacy migration), then implemented in PR #106.

**Implementation shape:**

- `internal/proxy/usagelog.go` — new `usageFilePathForDate(t)` returning `<configdir>/usage-YYYY-MM-DD.jsonl` (local date, matching how `ComputeGroup*Minutes` derives day boundaries from `t.Location()`); `parseUsageFileDate` rejects unrelated names (importantly the legacy `usage.jsonl`, since prune walks the directory and must not touch it); `listUsageFiles` returns matching files oldest-first by encoded date.
- `AppendUsageEvent` opens the file for the event's date in append mode. No file-level lock — appends from the DNS proxy and the scheduler are atomic at the kernel level for writes ≤PIPE_BUF, same property the original single-file code relied on.
- `ReadUsageEventsSince(since)` enumerates per-day files, drops files whose encoded date is strictly before `since`'s calendar day at the directory layer (cheap), then per-event filtering inside the boundary day. Read cost now scales with the requested range, not retention.
- `PruneOldUsageEvents(maxAge)` parses date from each filename and `os.Remove`s strictly-older-than-cutoffDay files. No file rewriting, no race with appends.
- `MigrateLegacyUsageFile()` opens `usage.jsonl` if present, splits lines into a `map[stamp][]string` keyed by event date, appends each bucket to its per-day file, removes the original on success. Idempotent: missing legacy file = no-op. Hooked into `program.run()` and the `--test-web` startup path right after `config.LoadConfig`.

**Retention cut:**

`scheduler.go` constant `60 * 24 * time.Hour` → `30 * 24 * time.Hour`. Dropped the "60 days" button from the dashboard's range picker (`internal/web/static/index.html`) and the `case "60d"` branch from `UsageHandler` (`internal/web/server.go`) — anything older than 30 days would have been pruned anyway, so the option was misleading. Updated DESIGN.md retention paragraph (now reads "30 days" + per-day filename pattern), README's `/api/usage` row, and a TROUBLESHOOTING note that referenced `usage.jsonl` by name.

**Tests:**

`internal/proxy/usagelog_test.go` (new) — six cases: path round-trip through `parseUsageFileDate`; rejection of unrelated filenames including the legacy `usage.jsonl`, malformed dates, and wrong extensions (load-bearing — prune walks the directory); append-then-read across a day boundary asserting two files exist oldest-first and the `since` filter returns the day-N tail plus all of day-N+1 in chronological order; prune at 30d removes -45d but keeps the -30d cutoff-edge file (cutoff inclusive on the keep side) and -1d/today; prune ignores `config.json`/`events.jsonl`/legacy `usage.jsonl`/random files even with retention pressure; migration of a multi-day legacy file produces three per-day files with correct contents and a second migration run is a no-op.

`make test` and `make build` green. Existing proxy/scheduler/web tests untouched and still passing.

**Gotchas worth flagging:**

- **Date stamp is local time, not UTC.** Matches the bucket-day computation in `ComputeGroup*Minutes`. Both filename and bucket-day key off `t.Location()` so they can't disagree, but a user crossing a timezone change would see a brief cosmetic discontinuity in file naming around the transition.
- **Legacy file is never a prune candidate.** `parseUsageFileDate` rejects the bare name `usage.jsonl`, so `PruneOldUsageEvents` walks past it. Lifecycle is owned by `MigrateLegacyUsageFile`, which removes it on first successful migration. If migration ever partially fails (some per-day files written, some not) the legacy file is preserved so the next start can retry — `O_APPEND` semantics mean the retry won't duplicate or lose events.
- **CI vs the 0-byte file watcher.** When polling `gh release view --json isDraft` for the publish notification, used `until ... ; do sleep 30 ; done` so the wakeup arrives via the harness's task notification rather than spam-polling — saved a chunk of cache-warm budget on what turned out to be a ~3 minute build.

**Wrap-up:** PR [#106](https://github.com/vsangava/sentinel/pull/106) merged to `main`; tag `v0.1.22` pushed and the release workflow published `sentinel-macos-arm64`, `sentinel-macos-amd64`, `sentinel-windows-amd64.exe`, and `install.sh` to <https://github.com/vsangava/sentinel/releases/tag/v0.1.22>. Foreground tracking continues to behave as designed (the original report was a misunderstanding of the metric, not a bug); usage logs are now per-day-rotated with 30-day retention, scaling read cost with the requested range and turning prune into an `unlink`.

## May 8 — Named configuration profiles (issue #50) → v0.1.23
**Session ID:** `feat-issue-50-named-profiles` · **PR:** [#107](https://github.com/vsangava/sentinel/pull/107)

**Opening prompt:**
> "let's start working on feature 50. it is probably a big feature, so make a plan, update the todo list to issue so if we happen to pause in the middle, we can pick up efficiently."

**What happened:**

Issue #50 had been sitting open: Sentinel has one config, and a user with distinct daily modes (deep work / casual evening / weekend) had to hand-edit JSON to switch between them. The user wanted profiles, scoped as a single big feature, but with the work checkpointed so it could be paused mid-flight.

Spent the first turn on understanding rather than acting — read the issue, launched two Explore agents in parallel (one on `internal/config` storage flow, one on `internal/web` endpoint layout) so the codebase context wasn't reconstructed from grep. The agents came back with the load/save call graph, every endpoint's auth pattern, the index.html header structure, and confirmation that no profiles concept existed today. Asked one clarifying question via `AskUserQuestion` — "single-file with inline profiles" vs. "multi-file (per issue spec)" vs. "multi-file with per-profile settings" — to pin down the disk layout before writing any code. The user's reply made the design call: *"see if group definitions can be reused across profiles."* That single sentence drove the whole architecture: bootstrap (`sentinel.json`) carries settings, the shared groups dictionary, pause/pomodoro, auth_token, and `active_profile`; each profile file (`profiles/<name>.json`) carries only `Rules`. Pause/Pomodoro stay bootstrap-level on purpose so a profile switch never breaks an active focus session, and the auth token stays put so the dashboard's stored credentials survive a switch. Wrote the plan to `~/.claude/plans/let-s-start-working-on-squishy-blanket.md` with six independently-shippable groups, posted a checklist comment to issue #50 mirroring those groups, then `ExitPlanMode` for approval.

User said "go" — the rest of the session was executing the six groups, ticking the issue checklist after each one so a future Claude could resume at the next unchecked box if the session got interrupted.

**Group 1 — storage layer (commit `955f0e2`).** Wrote `internal/config/profiles.go` with the bootstrap/profile types, name validation (`^[a-z0-9][a-z0-9_-]{0,31}$` + reserved names `sentinel`/`bootstrap`/`config`), atomic write-rename (CreateTemp + Rename) so a scheduler tick reading mid-write can't see a truncated file, `migrateLegacyConfigIfNeeded()` that splits legacy `config.json` and renames the original to `config.json.bak` (idempotent), and the public profile API (`ListProfiles`, `ProfileExists`, `CreateProfile(name, cloneFrom)`, `DeleteProfile`, `SwitchProfile`, `ReplaceFullConfig`, `ActiveProfile`). Refactored `LoadConfig`/`SaveConfig` in `config.go` to merge from / split into the two files. Crucially, the in-memory `Config` struct stayed identical — that single decision meant the scheduler, proxy, and enforcer packages saw zero changes. Added 12 unit tests covering fresh install, legacy migration round-trip, auth-token preservation across loads, switch round-trip, missing-active-profile fallback to default, and create/clone/delete edge cases.

A subtle catch surfaced: `TestStatusHandler_IncludesPomodoroState` in `internal/web/server_test.go` started failing after the storage refactor. The test relied on a quirk of the legacy load path — `json.Unmarshal` into an existing `AppConfig` left fields not in the JSON untouched, so an in-memory `Pomodoro` set by `setWorkPhase(t)` survived a subsequent `LoadConfig()` from disk. My new code reassigns `AppConfig = mergeBootstrapAndProfile(boot, prof)` cleanly, exposing the test helper's reliance on undocumented behaviour. Right fix wasn't to preserve the quirk in `LoadConfig` (semantically wrong — disk is meant to be source of truth) but to fix the helper: `setWorkPhase` / `setBreakPhase` now `LoadConfig` first to bootstrap state, then `SaveConfig` after mutating, with cleanup that `ClearPomodoro` + saves. Matches every real handler's flow and is robust to the new clean assignment.

**Group 2 — profile management API (commit `534ecb5`).** `GET /api/profiles` returns `{active, profiles[]}`. `POST /api/profiles` creates with optional `clone_from`; rejects with 400 on validation, 409 on duplicate. `DELETE /api/profiles/{name}` returns 404/409 on missing or active. `POST /api/profile/switch` is locked behind a Pomodoro work phase (HTTP 423 with the same wording style as `/api/pomodoro` DELETE and `/api/config/update`). Used `http.ServeMux` prefix routing for `/api/profiles/` to extract the name. Routed in both `StartWebServer` and `StartTestWebServer` so `--test-web` exercises the full surface. Updated `UpdateConfigHandler` to route through `config.ReplaceFullConfig` instead of writing to a single legacy path (which is gone — `GetConfigFilePath` retired in favour of `EnsureConfigDir`). Also extended `/api/status` to include `active_profile` so the dashboard could render a badge without an extra round-trip. Nine handler tests covering the full status-code matrix.

**Group 3 — CLI (commit `37c9021`).** Mirrored the existing `--set-mode` pattern in `cmd/app/main.go`: load → mutate → save → exit. `--set-profile <name>` calls `SwitchProfile`; on a bad name it lists the available profiles in the error so the user doesn't have to grep around for which they meant. `--list-profiles` prints names with `*` next to the active one. Both target the system bootstrap, so they require sudo — same constraint as `--set-mode`, and intentionally so (these are admin operations).

**Group 4 — dashboard UI (commit `0e9d744`).** Added a profile dropdown to the header (visible across all tabs, not buried in Manage), wired to a new `handleProfileSwitch(name)` that POSTs to `/api/profile/switch`, shows a success/error toast (`Switched to "<name>" — applies within 60s`), and refreshes status. The dropdown auto-disables during a Pomodoro work session — same affordance as the Pause button under the same condition, so the UI never lets the user attempt a 423-blocked operation. The Manage tab gained a Profiles section (gated by the existing `manageUnlocked` PIN flag) with a per-row Activate/Delete control and a Create form with optional clone-from. Status tab gets a small `⚑ Profile: <name>` badge under the enforcement-mode badge. Reused the existing bottom-of-script `escapeHtml`; added a small `escapeJs` helper for inline `onclick` handlers that splice profile names into JS string literals (handles the apostrophe edge). One annoyance: a real Sentinel service was running as root holding port 8040, so I couldn't spin up `--test-web` alongside it for live UI verification. Did a static brackets-balance check on the JS instead and deferred visual confirmation to Group 6.

**Group 5 — documentation (commit `84ae866`).** Followed the project convention of updating README, DESIGN, the landing page, and TROUBLESHOOTING in lockstep with code changes. README got a new "Profiles" section (added to ToC), an updated Configuration table showing the bootstrap + profile-directory split, the example config rewritten as two files matching what's actually written to disk, and CLI table additions for `--set-profile` / `--list-profiles`. DESIGN.md got the data-flow diagram updated, the `/api/profiles*` routes added to the web-server endpoint inventory, and §8 Configuration extended with a multi-row file-location table, a Profiles subsection inventorying every public function, and a Legacy migration subsection. The landing page got a "Named profiles" feature card with a folder-tab icon and a FAQ entry directly answering the issue's premise question ("Can I have different rules for work vs. weekend?"). TROUBLESHOOTING got four new entries: "Active profile points at a missing file" (the self-heal behaviour), "profile name invalid" (regex + reserved names), "Profile delete returns 409" (active-profile guard + permanently-undeletable default), and "Migration from a pre-profiles install" (rollback recipe via `config.json.bak`). Inline path/file references throughout TROUBLESHOOTING flipped from `config.json` to `sentinel.json` where the field in question lives in the bootstrap.

**Group 6 — verification.** `go test ./...` green across all eight packages. `make build-all` produced darwin-arm64, darwin-amd64, and windows-amd64 binaries cleanly. End-to-end smoke against `--test-web` in a tempdir with a seeded legacy `config.json` confirmed: migration produces `sentinel.json` + `profiles/default.json` + `config.json.bak` with the auth token preserved; create-with-clone returns 201, duplicate returns 409, invalid-name returns 400 with the regex echoed in the error, reserved-name returns 400; switch is reflected in `/api/status.active_profile`; deleting the active profile returns 409; starting a Pomodoro work phase blocks profile switching with 423 and the right error message; expiring the Pomodoro on disk + forcing a reload via `GET /api/config` unlocks the switch. The test ran against the same binary that would be released — same code path, same merged-file load, same atomic writes.

**Why one PR, not six.** Each group was a clean, independently-shippable commit, but bundling them into a single squashed PR matches recent project practice (PR #106, #103, #101) and avoids six separate review cycles for what is conceptually one feature. The per-group commits are still on the branch via the squash source if anyone wants to read the design rationale commit-by-commit. Issue #50's tracker comment was updated as each group landed, so the work was reviewable mid-flight even before the PR opened.

**Gotchas worth flagging:**

- **`UpdateConfigHandler` lost its direct `os.WriteFile`.** Legacy code path used `config.GetConfigFilePath()` to write the merged config to a single file. New code goes through `config.ReplaceFullConfig`, which preserves the auth token in-memory, splits across bootstrap + active profile, and reloads. Public `GetConfigFilePath` is gone (replaced by `EnsureConfigDir` for callers that just need the dir to exist); only `internal/web/server.go` referenced it.
- **Migration is one-shot but recoverable.** Once the new layout exists, `migrateLegacyConfigIfNeeded` is a no-op even if you re-introduce a `config.json` (the bootstrap is checked first). To roll back to an old binary, `config.json.bak` is restored and the new files deleted — TROUBLESHOOTING has the exact incantation. `.bak` is never auto-pruned.
- **Default profile is permanently undeletable.** Pragmatic choice: it's the migration target and the auto-fallback when `active_profile` points at a missing file. Renaming isn't supported either; clone + switch + delete the old one is the workflow. If a "rename" verb becomes valuable, it's a one-function add to `profiles.go`.
- **Profile name validation is server-side.** The dashboard's Create input limits to 32 chars but doesn't enforce the regex client-side — the server is the source of truth, and the 400 response includes the regex pattern in the error message, which the existing `showBanner` surfaces to the user. Considered duplicating the regex in JS; rejected as a sync hazard.
- **`--set-profile` requires sudo.** Same as `--set-mode`. There's deliberately no `--local` modifier today; for local-mode iteration the `--test-web` API surface is the right path. Adding env-var driven config-dir override (`SENTINEL_CONFIG_DIR`) would be a one-line addition to `configDir()` if a user wants ad-hoc local CLI testing.
- **Smoke testing through the live service.** When verifying the API end-to-end, the user had a real Sentinel daemon already on port 8040 (running as root). Killing it would interrupt their actual focus enforcement. Worked around by smoke-testing in a tempdir with `--test-web` only after confirming the port was free; deferred visual UI verification to first-deploy.

**Wrap-up:** PR [#107](https://github.com/vsangava/sentinel/pull/107) merged to `main`; tag `v0.1.23` pushed and the release workflow published `sentinel-macos-arm64`, `sentinel-macos-amd64`, `sentinel-windows-amd64.exe`, and `install.sh` to <https://github.com/vsangava/sentinel/releases/tag/v0.1.23>. Sentinel now supports named configuration profiles end-to-end: bootstrap + per-profile files on disk, migration from pre-v0.1.23 installs, dashboard dropdown + Manage section + Status badge, CLI flags, and Pomodoro-aware locking on switch. The first release where a single user can comfortably hold "Work" and "Weekend" rule sets without editing JSON.

## May 10 — Landing-page hero overflow on narrow viewports
**Session ID:** `docs-hero-mobile-responsive` · **PR:** [#108](https://github.com/vsangava/sentinel/pull/108)

**Opening prompt:**
> "on the documentation site, the top section above the section starting with 'OS-level blocking' doesn't seem to be responsive and the width is not changing in mobile browsers. can you check and fix?"

**What happened:**

Started by reading the structure of `docs/index.html` and locating the "OS-level blocking" anchor — the user meant the stats bar at line 180, putting the offending section squarely at the HERO. First hypothesis: the install one-liner. The hero's install snippet used `inline-flex … max-w-full` with a `<code class="truncate">` that lacked `min-w-0`, which is a classic flex bug — `truncate` adds `white-space: nowrap`, but the flex item's default `min-width: auto` pins it to its intrinsic content width, so the container can't shrink and the ellipsis never appears. Compared against the download section's snippet at line 711 (which uses `flex` + `flex-1` on the `<code>` — flex-1's `flex-basis: 0%` implies the same shrinking that `min-w-0` would). Applied the fix: `inline-flex max-w-full` → `flex w-full max-w-xl`, plus `flex-1 min-w-0 text-left` on the code. Reported back and the user confirmed it was much better but said the hero snippet "is not reducing in width to the same extent as the download snippet — only when the width is close to the min width."

That second report didn't match what the patch was supposed to do, so this turned into a real debug. Started a local `python -m http.server`, drove headless Chrome at various widths (`--window-size=360,1400`), and read back the screenshots. At 360px the hero content was visibly shifted right and clipped — content was wider than the viewport. The section's `overflow-hidden` was masking the layout: visually clipped, but the layout was wider than viewport so users could see content cut off.

Injected a temporary debug script into `<head>` that walked the DOM and listed every element whose bounding rect exceeded the viewport, then screenshotted again. The output revealed the actual chain: the hero `<section>` is `flex items-center` (used for vertical centering of its child), which makes the child div content-sized as a flex item rather than viewport-filling like a normal block child. Combined with `w-full max-w-xl` on the snippet, the snippet's max-content width (576px = max-w-xl) propagated up through the column-flex parent, through the hero's inner div (which sized itself to the widest child's max-content), and pinned the inner div at 608px (576 + 32px padding). At a 500px CSS viewport, the inner div would *not* shrink — flex-shrink couldn't take it below the content's intrinsic min-width, and the download section worked fine because its parent section is plain block, so the inner div fills the viewport directly.

Two changes shipped: add `w-full` to the hero inner div (forces viewport-fill regardless of flex sizing), and switch the snippet's max-w from `max-w-xl` (576) to `max-w-[40rem]` (640). 40rem matches the download section's effective max width exactly — its `max-w-2xl` parent (672) minus `px-4` (32) = 640 — so both snippets now reduce in lockstep at every breakpoint, which is what the user actually wanted.

Also addressed a secondary thing the user mentioned: `python -m http.server` was logging a 404 for `/favicon.ico`. The page had no `<link rel="icon">`, so browsers fell back to the conventional path. GitHub Pages serves a default favicon to mask this, so it's only visible during local testing. Added `<link rel="icon" type="image/png" href="assets/img/logo2.png">` pointing at the only logo asset present.

Split into two commits — responsiveness fix, then favicon — pushed `docs/hero-mobile-responsive`, opened PR #108, waited for CI green, squash-merged with branch delete.

**Gotchas worth flagging:**

- **`overflow-hidden` masks layout overflow visually but the layout is still wider than the viewport.** A reader scrolling on a touch device sees clipped content, not a horizontal scrollbar, which is why the symptom presented as "not responsive" rather than "page is scrolling sideways." The fix is to remove the cause of the wide layout (the `max-w-xl` propagation), not to add more `overflow-hidden`.
- **Chrome headless ignores `--window-size` for the CSS viewport.** It always renders at ~500px CSS regardless of the flag and downscales the screenshot. The injected debug overlay's `window.innerWidth=500` confirmed this. Means automated narrow-viewport regression via Chrome headless flags alone isn't viable for this file — verify on a real phone or Chrome DevTools device mode.
- **`flex-1` and `min-w-0` are not interchangeable but solve the same problem.** `flex-1` sets `flex: 1 1 0%` (basis 0 implies the item can shrink); `min-w-0` lets a flex item shrink below its content's min-content width regardless of basis. Used both on the snippet's `<code>` belt-and-braces because the `w-full` parent path is unusual.
- **`max-w-[40rem]` is an arbitrary Tailwind value, intentionally.** Neither `max-w-xl` (576px) nor `max-w-2xl` (672px) matches the download section's 640px effective max. Kept it explicit so the symmetry with the download snippet is obvious in the diff — anyone touching either snippet later sees the matching values.

**Wrap-up:** PR [#108](https://github.com/vsangava/sentinel/pull/108) merged to `main`; no release. Hero install snippet now truncates correctly on narrow viewports, hero inner div fills the section width regardless of flex sizing, and both install snippets on the landing page (hero and download CTA) shrink in lockstep at every breakpoint. Favicon resolves to `assets/img/logo2.png`, suppressing the `/favicon.ico` 404 during local testing.

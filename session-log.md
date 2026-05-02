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

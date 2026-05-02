# Troubleshooting & Recovery

This document is for when something is wrong. If you're just installing for the first time, read [README.md](./README.md). If you want to understand how the daemon works internally, read [DESIGN.md](./DESIGN.md).

## Contents

1. [If your internet is broken — read this first](#1-if-your-internet-is-broken--read-this-first)
2. [Quick triage by symptom](#2-quick-triage-by-symptom)
3. [Inspecting service state](#3-inspecting-service-state)
4. [Mode-specific diagnostics](#4-mode-specific-diagnostics)
5. [Reading logs](#5-reading-logs)
6. [Manual install / start / stop / uninstall](#6-manual-install--start--stop--uninstall)
7. [Verifying behaviour without installing the service](#7-verifying-behaviour-without-installing-the-service)
8. [The `clean` recovery path](#8-the-clean-recovery-path)
9. [Common errors](#9-common-errors)

---

## 1. If your internet is broken — read this first

If you removed or stopped the service while it was in `dns` or `strict` mode and now nothing resolves, your system DNS is still pointing at `127.0.0.1` but the proxy isn't there to answer.

**Fix it in one command (requires sudo):**

```bash
sudo ./sentinel clean --yes
```

That iterates every network interface, resets the ones pointing at `127.0.0.1`, flushes the resolver cache, and verifies port 53 is free. It also handles the case where you've already deleted the binary's config dir.

**If you no longer have the binary**, do it by hand:

```bash
# macOS — list every interface, reset any whose DNS shows 127.0.0.1
networksetup -listallnetworkservices
networksetup -getdnsservers Wi-Fi          # check
networksetup -setdnsservers Wi-Fi Empty    # reset
networksetup -setdnsservers Ethernet Empty # repeat per interface
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder
```

```powershell
# Windows
Get-DnsClientServerAddress | Where-Object { $_.ServerAddresses -contains "127.0.0.1" } |
  ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }
ipconfig /flushdns
```

`hosts` mode (the default) does not change your DNS settings, so this scenario doesn't apply to it. If you're on `hosts` mode and resolution is broken, the cause is almost certainly an entry in `/etc/hosts` — see [section 4](#4-mode-specific-diagnostics).

---

## 2. Quick triage by symptom

| Symptom | Likely cause | Where to look |
|---|---|---|
| Blocked sites still load | Wrong mode for your setup, or current time isn't in a block window | [§4 Mode-specific diagnostics](#4-mode-specific-diagnostics) |
| Site blocked in DNS but loads in browser (strict mode) | CDN rotated to an IP not yet in pf rules, or pre-existing connection | [§4 strict mode — diagnosing "pf active but site still loads"](#diagnosing-pf-active-but-site-still-loads) |
| All DNS broken (nothing resolves) | DNS-mode service stopped without restoring system DNS | [§1](#1-if-your-internet-is-broken--read-this-first) |
| Service won't start: `permission denied` or `address already in use` on port 53 | Missing sudo, or another DNS service (AdGuard Home, dnsmasq…) holds port 53 | [§9 Port 53 errors](#port-53-errors-permission-denied--address-already-in-use) |
| Service installed but `start` does nothing | Service framework is silent about startup failures — check logs | [§5 Reading logs](#5-reading-logs) |
| Tabs aren't being closed | Not running as console user, AppleScript permissions, browser not running | [§4 macOS AppleScript path](#macos-applescript-path) |
| Web dashboard returns 401 unauthorized | Auth token mismatch — UI didn't bootstrap | [§9](#9-common-errors) |
| Config edits aren't taking effect | Bad JSON, or you didn't wait 60 seconds | [§9](#9-common-errors) |
| `/api/pause` returns 400 | `minutes` outside 1–240 range | — |
| `clean` reports critical failures | A step couldn't undo something — output tells you which | [§8 The clean recovery path](#8-the-clean-recovery-path) |

---

## 3. Inspecting service state

```bash
# Service registered & running?
sudo ./sentinel status

# What does the live daemon think is blocked right now?
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')
curl -s -H "X-Auth-Token: $TOKEN" http://localhost:8040/api/status | jq
# → { "blocked_domains": {...}, "last_evaluated": "...", "enforcement_mode": "hosts", "paused": false }

# What groups and rules are loaded?
curl -s http://localhost:8040/api/config | jq

# Would a specific (time, domain) be blocked?
# (Response includes the group name and which member of the group matched.)
curl -s -H "X-Auth-Token: $TOKEN" \
  "http://localhost:8040/api/test-query?time=2024-04-01%2010:30&domain=roblox.com" | jq
```

The same checks via the dashboard: open `http://localhost:8040`, look at the **Status** tab. The blocked-domains list and `last_evaluated` timestamp tell you whether the scheduler is ticking and what it last decided.

A `last_evaluated` more than ~70 seconds old means the tick loop has died. Restart the service.

---

## 4. Mode-specific diagnostics

What "blocking is broken" looks like depends on the active enforcement mode. Check it via:

```bash
curl -s http://localhost:8040/api/config | jq -r '.settings.enforcement_mode // "hosts"'
```

### `hosts` mode (default)

The daemon edits `/etc/hosts`. To see what it's currently writing:

```bash
# macOS / Linux
sed -n '/# sentinel:begin/,/# sentinel:end/p' /etc/hosts

# Windows (PowerShell)
Get-Content C:\Windows\System32\drivers\etc\hosts |
  Select-String -Pattern "sentinel:begin" -Context 0,100
```

If the block isn't there during a scheduled window, the scheduler hasn't fired (check `/api/status` `last_evaluated`).

If the block *is* there but the site loads anyway:

- Your browser or app may have cached the DNS resolution. Try a private window, or restart the app.
- The browser may be using DNS-over-HTTPS (DoH) or DNS-over-TLS (DoT), which bypasses `/etc/hosts`. **Disable DoH** in the browser, or switch to `dns`/`strict` mode (which also won't help against DoH unless you can intercept the upstream — `pf` in strict mode does, since it blocks the resolved IPs at the kernel).
- The site may be served from a CDN domain not covered by the static prefix list (`""`, `www.`, `m.`, `mobile.`, `app.`). Add the relevant subdomain to the relevant group in `config.json`.

To preview what *would* be written without root:

```bash
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')
curl -s -H "X-Auth-Token: $TOKEN" http://localhost:8040/api/hosts-preview | jq
```

### `dns` mode

Verify the proxy is up and your OS is using it:

```bash
# Is anything listening on UDP port 53?
sudo lsof -i :53 -P -n

# Is the system DNS pointing at 127.0.0.1?
networksetup -getdnsservers Wi-Fi
scutil --dns | grep nameserver

# Does the proxy actually return 0.0.0.0 for blocked domains?
dig @127.0.0.1 youtube.com
# → If blocked: ANSWER section shows youtube.com 60 IN A 0.0.0.0
# → If allowed: forwarded result from 8.8.8.8

# What does the system see (uses configured resolver)?
dig youtube.com
```

If `dig @127.0.0.1` returns `0.0.0.0` but `dig youtube.com` returns a real IP, the proxy is working but the OS DNS is not pointed at it. Reset:

```bash
networksetup -setdnsservers Wi-Fi 127.0.0.1
sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder
```

If `nslookup <domain>` says `Got recursion not available from 127.0.0.1, trying next server` and falls through to your backup DNS, the service is running an old binary that predates the RA-bit fix. Restart it with:

```bash
sudo ./sentinel stop && sudo ./sentinel start
```

See [§6 Restarting after a binary update](#restarting-after-a-binary-update) for why `launchctl stop/start` is not sufficient here.

### `strict` mode

Strict mode adds a pf (packet filter) firewall layer on top of DNS blocking. DNS alone can be bypassed by apps that have cached a real IP; pf drops packets to those IPs at the kernel level regardless. Both layers must be working for strict mode to be effective.

#### Layer 1: Verify DNS is blocking (same as `dns` mode)

```bash
# Is the proxy returning 0.0.0.0 for blocked domains?
dig @127.0.0.1 facebook.com +short
# → 0.0.0.0 means DNS layer is working

# Is the OS actually using our proxy?
networksetup -getdnsservers Wi-Fi
# → should include 127.0.0.1
```

#### Layer 2: Verify pf is active and loaded

```bash
# Is pf enabled?
sudo pfctl -s info | head -3
# → "Status: Enabled" — if "Disabled", pf never started (see below)

# Is our anchor registered?
sudo pfctl -s Anchors
# → "sentinel" should appear in the list

# What rules are active in our anchor?
sudo pfctl -a sentinel -s rules
# → Should show lines like:
#   block drop out quick inet proto tcp from any to <__automatic_xxxxxxxx_0>
#   block drop out quick inet proto udp from any to <__automatic_xxxxxxxx_1>
#   block drop out quick inet6 proto tcp from any to <__automatic_xxxxxxxx_2>
#   block drop out quick inet6 proto udp from any to <__automatic_xxxxxxxx_3>
# macOS automatically promotes inline IP lists to internal tables (__automatic_*).
# "No rules" means either no domains are currently blocked, or anchor loading failed.

# What IPs are in the active tables?
# (Replace the hash with what you see in "pfctl -a sentinel -s rules" above)
sudo pfctl -a sentinel -t __automatic_<hash>_0 -T show
# → Lists all blocked IPv4 addresses

sudo pfctl -a sentinel -t __automatic_<hash>_2 -T show
# → Lists all blocked IPv6 addresses
```

#### Checking if a specific IP is being blocked

```bash
# Resolve the current real IP for a domain (bypassing Sentinel's proxy):
dig facebook.com @1.1.1.1 +short
dig facebook.com @1.1.1.1 AAAA +short

# Then check if that IP is in the active pf table:
sudo pfctl -a sentinel -t __automatic_<hash>_0 -T show | grep <ip>
# → If the IP appears: pf should be blocking it. Try:
curl -v --max-time 5 https://facebook.com
# → Should hang/timeout with connection dropped

# → If the IP does NOT appear: pf doesn't know about this IP yet.
# The Refresh() loop re-resolves every tick (≤60s). Wait one minute and recheck.
# If it still doesn't appear, check the logs:
log show --predicate 'process == "sentinel"' --last 5m | grep pf
```

#### Diagnosing "pf active but site still loads"

**Step 1 — Is the IP in the table?**

If the IP the browser is connecting to is not in the table, pf won't block it. CDN-backed sites (Facebook, Twitter, Instagram) rotate through dozens of IPs. The daemon re-resolves every minute via `Refresh()`, but there is a ≤60 s window where a fresh IP isn't blocked yet.

```bash
# See what IP an active connection is actually using:
sudo pfctl -s states | grep 443 | grep <domain-ip-range>
```

**Step 2 — Check the anchor file on disk**

```bash
cat /etc/pf.anchors/sentinel
# Should look like:
#   block drop out quick inet proto {tcp udp} from any to { 1.2.3.4 5.6.7.8 }
#   block drop out quick inet6 proto {tcp udp} from any to { 2001:db8::1 }
# If it says "# no IPs to block", either no domains are scheduled or IP resolution failed.
```

**Step 3 — Check that pf.conf has our anchor**

```bash
grep -A3 "sentinel" /etc/pf.conf
# Should show:
#   # sentinel:begin
#   anchor "sentinel"
#   load anchor "sentinel" from "/etc/pf.anchors/sentinel"
#   # sentinel:end
# If absent, the anchor was never injected — check setup logs.
```

**Step 4 — Verify IP resolution is working**

Strict mode resolves IPs using `backup_dns` as a fallback when `primary_dns` (usually `127.0.0.1:53`, Sentinel's own proxy) returns `0.0.0.0` for blocked domains. If `backup_dns` is unset or wrong, no IPs will be resolved.

```bash
# What does the config say?
curl -s http://localhost:8040/api/config | jq '.settings | {primary_dns, backup_dns}'

# Manually resolve using backup_dns to see what IPs you expect:
dig facebook.com @1.1.1.1 +short      # replace 1.1.1.1 with your backup_dns host
dig facebook.com @1.1.1.1 AAAA +short

# Check log for IP resolution issues:
log show --predicate 'process == "sentinel"' --last 5m | grep "pf:"
# "pf: no IPs resolved" → backup_dns is failing or unreachable
# "pf: load anchor: ... syntax error" → anchor file has malformed rules
```

**Step 5 — Re-activate manually to force a reload**

If you need to force an immediate pf refresh without waiting for the next tick, restart the service:

```bash
sudo ./sentinel stop && sudo ./sentinel start
# Then wait ~5s for the first tick, and verify:
sudo pfctl -a sentinel -s rules
```

#### Known pf limitations

- **CDN IP rotation**: Sites like Facebook serve from large IP pools and rotate constantly. There is always a ≤60 s window between a new IP appearing and Sentinel blocking it. Closing and reopening the browser tab forces a fresh connection through the updated rules.
- **Pre-existing connections**: `pfctl -k` is run to kill existing states when a block activates, but connections opened before Sentinel started aren't guaranteed to be killed. Restart the browser if a site that should be blocked remains reachable.
- **IPv6 must be covered**: Browsers will prefer IPv6 if available. If only IPv4 IPs are in the anchor, a site reachable via IPv6 will bypass the block. Sentinel resolves both A and AAAA records — verify both tables are populated.

If `Setup` failed (logs show `pf anchor setup failed`), the strict enforcer degrades to DNS-only. That's intentional — better to keep blocking at the DNS layer than crash the daemon. Look in the daemon logs for the specific pfctl error, then run `sudo ./sentinel clean --yes && sudo ./sentinel install && sudo ./sentinel start` to reset.

### macOS AppleScript path

The 3-minute warning notification and the tab-closing only fire on macOS, only via `osascript`. If they're not happening:

```bash
# Test the script generation + execution by hand (no service install needed)
./sentinel --test-applescript
```

Common gotchas:

- **Running as root via launchd, but no console user detected.** `osascript` invoked as root with no user context produces a notification nobody can see. The daemon detects the console user via `stat -f %Su /dev/console` and shells out via `su - <user> -c osascript ...`. If `/dev/console` returns `root` (no one logged in), notifications are silently skipped.
- **macOS Automation permissions.** First run, macOS prompts: *"sentinel wants to control 'Google Chrome'."* If you click Don't Allow, AppleScript silently fails forever after. Reset in System Settings → Privacy & Security → Automation.
- **Browsers not running.** The warning script first checks for open tabs; if no Chrome or Safari window matches, the notification is suppressed by design.

---

## 5. Reading logs

### macOS

The daemon logs to stdout/stderr; launchd routes that to the system log:

```bash
# Live tail
log stream --predicate 'process == "sentinel"' --level debug

# Last hour
log show --predicate 'process == "sentinel"' --last 1h

# Just the scheduler ticks
log show --predicate 'process == "sentinel"' --last 1h | grep -E "scheduler|hosts|dns"
```

If the service was installed via `kardianos/service` defaults, the launchd plist is at `~/Library/LaunchAgents/com.github.sentinel.plist`:

```bash
cat ~/Library/LaunchAgents/com.github.sentinel.plist
launchctl print system/com.github.sentinel
```

### Windows

```powershell
Get-EventLog -LogName Application -Source "Sentinel" -Newest 50
# Service status
Get-Service "Sentinel"
```

### Anywhere

If launchd / Service Manager logs aren't useful, run the daemon in the foreground to see everything live:

```bash
sudo ./sentinel stop          # stop the service version
sudo ./sentinel run           # run with the supervisor (foreground)
# or, even simpler, no privileges needed for hosts-mode rule evaluation:
./sentinel --local            # uses ./config.json
```

`--local` is a fast way to validate that the rules and scheduler logic work — it skips system paths and won't install anything.

---

## 6. Manual install / start / stop / uninstall

The normal happy path is `sudo ./sentinel install && sudo ./sentinel start`. If something goes wrong, here's how to verify each step independently.

```bash
sudo ./sentinel install
# Verify on macOS:
ls -la ~/Library/LaunchAgents/com.github.sentinel.plist
launchctl list | grep sentinel
```

```bash
sudo ./sentinel start
# Verify:
sudo ./sentinel status         # Should print: running
ps aux | grep sentinel | grep -v grep
sudo lsof -i :53 -P -n                  # only relevant in dns/strict mode
sudo lsof -i :8040 -P -n                # web dashboard
curl -s http://localhost:8040/api/config | jq -r '.settings.enforcement_mode'
```

```bash
sudo ./sentinel stop
# Verify (mode-dependent):
#   hosts: managed block removed from /etc/hosts
#   dns:   networksetup -getdnsservers Wi-Fi → restored to upstream
#   strict: above + pf anchor removed (sudo pfctl -s Anchors)
```

```bash
sudo ./sentinel uninstall
# Verify:
ls ~/Library/LaunchAgents/com.github.sentinel.plist  # should be gone
launchctl list | grep sentinel                       # should be empty
```

#### Restarting after a binary update

Always use `sudo ./sentinel stop && sudo ./sentinel start` — not raw `launchctl stop/start`.

`launchctl stop <label>` sends SIGTERM, but with `KeepAlive: true` in the plist launchd immediately relaunches the process. Running `launchctl start` on top of that races the KeepAlive restart and can silently no-op. The `./sentinel stop/start` path calls `launchctl unload`/`launchctl load` through the `kardianos/service` library, which fully unregisters and re-registers the job — guaranteeing the new binary is exec'd.

---

## 7. Verifying behaviour without installing the service

Three flags exist exactly so you can test the daemon's core logic without privileges, without binding port 53, and without modifying the system.

```bash
# Will youtube.com be blocked at this specific time per current ./config.json?
./sentinel --test-query "2024-04-01 10:30" youtube.com
# Output includes: applicable rules, would-block status, and a real DNS response
# from upstream so you can verify what the upstream resolver returns.

# Run the full dashboard against ./config.json (no service install, no system changes).
# All endpoints work, including hosts-preview and pf-preview.
./sentinel --test-web
# → http://localhost:8040

# Generate (and optionally execute) the AppleScript that closes Chrome/Safari tabs.
./sentinel --test-applescript
```

`--test-query` and `--test-web` use `./config.json` in the working directory rather than the system path, so you can iterate on groups and rules without touching the live config. `make build` followed by `--test-web` is the fastest dev loop for trying out new group shapes or schedule timings.

---

## 8. The `clean` recovery path

`clean` is the canonical "make it like sentinel was never installed" command. Run it any time the system is in an unknown state — it does not assume `stop` succeeded, and every step is idempotent.

```bash
sudo ./sentinel clean         # interactive: prompts before deleting config dir
sudo ./sentinel clean --yes   # non-interactive
```

The output is one line per step:

```
[✓] Service stopped
[✓] DNS on Wi-Fi
[—] DNS on Bluetooth PAN: already Empty
[✓] DNS on Ethernet
[✓] Clean /etc/hosts
[✓] Remove pf anchor
[✓] Flush DNS cache
[✓] Service uninstalled
[✓] Remove config directory
[—] Remove temp files: none found
[✓] Port 53 is free
```

Status meanings:

| Icon | Meaning |
|---|---|
| `[✓]` done | Step completed successfully |
| `[—]` skipped | Step was a no-op (already in the desired state, or not applicable on this OS) |
| `[!]` warn | Non-critical failure; cleanup continues |
| `[✗]` error | Critical failure; if any are present `clean` exits non-zero |

If `Port 53 is free` reports a warning, something unrelated is holding the port:

```bash
sudo lsof -i :53 -P -n
# Find the PID, decide whether to kill it.
```

If `Reset DNS interfaces` fails on macOS, `networksetup -listallnetworkservices` may be returning interfaces with unusual names. Reset them by hand:

```bash
networksetup -listallnetworkservices
for svc in "Wi-Fi" "Ethernet" "Thunderbolt Bridge"; do
  networksetup -getdnsservers "$svc"
done
networksetup -setdnsservers "Wi-Fi" Empty
```

---

## 9. Common errors

### Port 53 errors (`permission denied` / `address already in use`)

These only apply to `dns` and `strict` modes. In `hosts` mode the daemon never binds port 53.

**`permission denied`** — port 53 is privileged; run with `sudo`.

**`address already in use`** — another process is already listening on port 53. Sentinel logs the name of the conflicting process automatically. You can also find it manually:

```bash
sudo lsof -i :53 -P -n
```

Common culprits: AdGuard Home, Pi-hole, systemd-resolved, dnsmasq, or another Sentinel instance. Decide whether to stop it or let Sentinel share upstream with it (see below).

#### Running Sentinel alongside AdGuard Home (or any other local DNS service)

Sentinel needs to own port 53 to intercept queries. The other service must move to a different port so Sentinel can forward to it as upstream.

**Step 1 — Move the other service off port 53.**

*AdGuard Home:* Settings → DNS Settings → "DNS server configuration" → change the plain DNS port (default 53) to something like **5300**. Save and apply. AdGuard continues filtering ads/tracking; it just listens on a different port.

*Pi-hole / dnsmasq:* edit the service's config to bind to a non-standard port (e.g. 5300), then restart it.

**Step 2 — Point Sentinel's upstream at the other service.**

```bash
# Open the web UI and change primary_dns to 127.0.0.1:5300
open http://localhost:8040
# — or edit config.json directly —
sudo nano /Library/Application\ Support/Sentinel/config.json
# set "primary_dns": "127.0.0.1:5300"
```

You can also set a backup in case your local resolver is down:

```json
"primary_dns": "127.0.0.1:5300",
"backup_dns":  "8.8.8.8:53"
```

> **Note:** If your `primary_dns` was still at the factory default (`8.8.8.8:53`) when Sentinel first started in `dns` or `strict` mode, Sentinel auto-detected your previous system DNS and saved it automatically. In that case this step may already be done — check the web UI or config.json before editing.

**Step 3 — Restart Sentinel.**

```bash
sudo sentinel restart
```

Sentinel now owns port 53 (blocking distracting sites by returning `0.0.0.0`) and forwards all other queries to AdGuard Home, which continues its own ad/tracker filtering.

#### What happens if Sentinel crashes or is killed?

In `dns`/`strict` mode the OS sends all DNS through Sentinel. If it stops unexpectedly, behaviour depends on `dns_failure_mode` in `config.json`:

| `dns_failure_mode` | Sentinel down | Sentinel restarted by launchd |
|--------------------|--------------|-------------------------------|
| `"open"` (default) | Machine falls back to `backup_dns` — internet works, blocking lapses | Sentinel takes over again within seconds |
| `"closed"` | DNS resolution fails entirely — no internet until Sentinel is back | Same |

**`"open"`** is the default. It requires `backup_dns` to be a non-loopback IP on port 53 (e.g. `1.1.1.1:53`). If you have pointed `backup_dns` at a local resolver on a non-standard port (e.g. `127.0.0.1:5300` for AdGuard), Sentinel cannot use it as an OS-level fallback and will log a warning; it will operate fail-closed in that case regardless of the setting.

**`"closed"`** is appropriate when you need the blocking to be unbypassable — a crash means no internet rather than unfiltered internet. Be aware that a motivated user could potentially trigger a crash to temporarily lift blocks.

To change the setting:

```bash
sudo nano /Library/Application\ Support/Sentinel/config.json
# set "dns_failure_mode": "closed"
sudo sentinel restart
```

### `service is already installed`

```bash
sudo ./sentinel uninstall
sudo ./sentinel install
```

If `uninstall` claims it isn't installed, the plist is dangling:

```bash
launchctl unload ~/Library/LaunchAgents/com.github.sentinel.plist
rm ~/Library/LaunchAgents/com.github.sentinel.plist
sudo ./sentinel install
```

### `cannot uninstall while service is running`

```bash
sudo ./sentinel stop
sudo ./sentinel uninstall
```

If `stop` hangs or fails, force it:

```bash
sudo pkill -f "sentinel"
launchctl unload ~/Library/LaunchAgents/com.github.sentinel.plist
sudo ./sentinel uninstall
```

### Web dashboard returns 401 unauthorized

The dashboard fetches the auth token from `GET /api/config` on first load. If you see 401s on `/api/status`, `/api/test-query`, etc., the JS didn't pick up the token. Hard-refresh the page (`Cmd+Shift+R`).

If you're hitting the API from `curl`, set the header explicitly:

```bash
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')
curl -H "X-Auth-Token: $TOKEN" http://localhost:8040/api/status
```

### Config edits aren't taking effect

The scheduler reloads `config.json` once per minute. Wait 60 seconds.

If they're still not applied:

```bash
# Validate JSON
python3 -m json.tool < /Library/Application\ Support/Sentinel/config.json

# Check the daemon's logs for parse errors
log show --predicate 'process == "sentinel"' --last 5m | grep -i config
```

If the file is malformed, the daemon logs a warning and keeps using the previous in-memory copy — your changes won't apply until you fix the JSON.

You can also force a reload by hitting `GET /api/config` (which calls `LoadConfig()`):

```bash
curl -s http://localhost:8040/api/config > /dev/null
```

### `pf anchor setup failed` in logs (strict mode)

The strict enforcer degrades to DNS-only when pf setup fails. Common causes:

- `/etc/pf.conf` is non-standard and the daemon doesn't recognise it.
- Another tool has its own anchors and is fighting for control.
- pf is disabled in a way that `pfctl -e` can't fix.

To start fresh:

```bash
sudo ./sentinel clean --yes   # removes our anchor + pf.conf injection
# inspect /etc/pf.conf — should be back to its original state
sudo ./sentinel install
sudo ./sentinel start
# logs should show "pf: anchor installed"
```

### Tabs don't close on block start

See [§4 macOS AppleScript path](#macos-applescript-path). The most common cause is denied automation permissions for Chrome/Safari — fix in System Settings → Privacy & Security → Automation.

### `clean` says "could not create service handle"

Usually means the binary you're running was built for a different OS. Check `file ./sentinel` and rebuild for the current platform with `make build`.

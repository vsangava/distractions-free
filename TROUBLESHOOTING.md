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
| All DNS broken (nothing resolves) | DNS-mode service stopped without restoring system DNS | [§1](#1-if-your-internet-is-broken--read-this-first) |
| Service won't start: `permission denied` on port 53 | Running without sudo, or another process holds port 53 | [§9](#9-common-errors) |
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

### `strict` mode

Strict mode = DNS mode + pf. Verify both layers:

```bash
# pf enabled?
sudo pfctl -s info | head -3
# → Status: Enabled

# Our anchor loaded?
sudo pfctl -s Anchors
# → sentinel should appear

# What IPs are currently blocked at the firewall?
sudo pfctl -a sentinel -t blocked_ips -T show

# Preview what strict mode would produce for current blocks (no root needed)
TOKEN=$(curl -s http://localhost:8040/api/config | jq -r '.settings.auth_token')
curl -s -H "X-Auth-Token: $TOKEN" http://localhost:8040/api/pf-preview | jq
```

If `Setup` failed (logs show `pf anchor setup failed`), the strict enforcer is degraded to DNS-only. That's intentional — better to keep blocking at the DNS layer than crash the daemon. Look in the daemon logs for the specific pfctl error.

If pf is loaded but the site still loads:

- The CDN may have rotated to a fresh IP not in your table. The next scheduler tick (within 60 s) re-resolves and reloads.
- Existing TCP connections survive an anchor reload by default — the daemon does run `pfctl -k` to kill matching states, but a connection that was opened *before* the IP made it onto the table is unaffected. Close the browser and retry.

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

### `listen udp 127.0.0.1:53: permission denied`

Port 53 is privileged. Run with `sudo`. If you *are* using sudo, something else holds the port:

```bash
sudo lsof -i :53 -P -n
sudo kill <PID>           # only if you know what it is
```

This error only applies to `dns` and `strict` modes. In `hosts` mode the daemon doesn't bind any port (besides 8040 for the dashboard, which is unprivileged).

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

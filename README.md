# 🚫 Distractions-Free

A lightweight, system-level background daemon for macOS (and Windows) that enforces productivity schedules by acting as a local DNS proxy. 

Unlike browser extensions that can be easily disabled, **Distractions-Free** runs as a root system service, intercepts DNS requests to distracting domains, and seamlessly kills active browser tabs when a focus block begins.

## ✨ Features
* **Local DNS Blackholing**: Blocks domains instantly at the OS level by returning \`0.0.0.0\`.
* **Zero-CPU Polling**: Smart 1-minute scheduling that consumes 0% CPU and handles laptop sleep/wake cycles flawlessly.
* **Intelligent Tab Killer (macOS)**: Native AppleScript integration closes blocked tabs automatically in Chrome and Safari when a schedule triggers.
* **3-Minute Warnings**: Sends native macOS notifications as the logged-in user right before a block starts.
* **Embedded Dashboard**: Ships with an embedded web UI and JSON API (no external web assets required).
* **System Service Integration**: Installs itself automatically as a \`launchd\` daemon on macOS to survive reboots.

### Platform feature support

| Feature | macOS | Windows |
|---|---|---|
| DNS blocking | ✅ | ✅ |
| Automatic browser tab closing | ✅ (Chrome, Safari via AppleScript) | ❌ |
| Pre-block warning notifications | ✅ (native macOS notifications) | ❌ |
| System service (auto-start on boot) | ✅ (launchd) | ✅ (Windows Service) |

---

## ⚡ Quick Download

**Don't want to compile?** Download pre-built binaries from the latest [GitHub Release](https://github.com/vsangava/distractions-free/releases):

- 🍎 **macOS Apple Silicon** (M1/M2/M3): `distractions-free-macos-arm64`
- 🪟 **Windows** (x86_64): `distractions-free-windows-amd64.exe`

Then skip to **Step 2** in the installation guide below.

---

## 🛠 Prerequisites (For Building from Source)

If you want to compile the binary yourself, you need the Go compiler installed:
``` bash
brew install go
```

---

## 🚀 Installation & Setup

### Step 1: Get the Binary

**Option A: Download Pre-built Binary** (Recommended)
1. Go to [GitHub Releases](https://github.com/vsangava/distractions-free/releases)
2. Download the binary for your system
3. Make it executable: `chmod +x distractions-free-macos-*`

**Option B: Build from Source**
Clone and build:
``` bash
git clone https://github.com/vsangava/distractions-free.git
cd distractions-free
go build -o distractions-free ./cmd/app
```

### Step 2: Install the Background Service
Because this app runs a local DNS server on port 53, it requires Root/Administrator privileges.
``` bash
sudo ./distractions-free install
```

### Step 3: Start the Service
``` bash
sudo ./distractions-free start
```

### Step 4: Point Your OS to the Proxy
Tell macOS to use your new local DNS proxy instead of your router's default DNS.
``` bash
# If using Wi-Fi:
networksetup -setdnsservers Wi-Fi 127.0.0.1

# If using Ethernet:
networksetup -setdnsservers Ethernet 127.0.0.1
```

---

## ⚙️ Administration & Configuration

### The Web Dashboard
Once the service is running, you can access the local dashboard and API via:
👉 **http://localhost:8040**

### Modifying the Schedule
By default, the daemon stores its configuration file at a secure, absolute system path:
* **macOS:** \`/Library/Application Support/DistractionsFree/config.json\`
* **Windows:** \`C:\ProgramData\DistractionsFree\config.json\`

To update your rules, simply edit this file. The background daemon will automatically detect and apply the new rules on the next 1-minute tick!

**Example Configuration:**
``` json
{
  "settings": {
    "primary_dns": "8.8.8.8:53",
    "backup_dns": "1.1.1.1:53"
  },
  "rules": [
    {
      "domain": "facebook.com",
      "is_active": true,
      "schedules": {
        "Monday": [
          {"start": "09:00", "end": "11:00"},
          {"start": "13:00", "end": "15:00"},
          {"start": "16:00", "end": "17:00"}
        ],
        "Wednesday": [
          {"start": "09:00", "end": "12:00"},
          {"start": "14:00", "end": "17:00"}
        ]
      }
    }
  ]
}
```

---

## 🛑 Uninstallation & Safety

**⚠️ CRITICAL:** Do not delete the application without restoring your system DNS settings first, or your internet will stop working!

To safely remove the app, run the built-in uninstall commands. Our daemon is programmed to automatically restore your default macOS Wi-Fi DNS and flush your cache upon shutdown:
``` bash
# 1. Stop the service (Automatically restores default OS DNS)
sudo ./distractions-free stop

# 2. Remove the daemon from system startup
sudo ./distractions-free uninstall

# 3. Clean up the configuration folder
sudo rm -rf "/Library/Application Support/DistractionsFree"

---

## 🧪 Testing

The codebase includes comprehensive unit tests for the core blocking and scheduling logic that run **without requiring privileges, port binding, or system modifications**.

### Run All Tests
``` bash
go test ./internal/... -v
```

### Run Specific Test Suites

**Scheduler Tests** (time-based blocking rules evaluation):
``` bash
go test ./internal/scheduler -v
```
- Tests rule evaluation at specific times
- Validates blocking windows (start/end times)
- Tests warning triggers (3-minute pre-block notifications)
- Tests all weekday schedules and edge cases

**DNS Proxy Tests** (DNS request handling and forwarding):
``` bash
go test ./internal/proxy -v
```
- Tests blocked domain responses (returns `0.0.0.0`)
- Tests allowed domain forwarding to **real upstream DNS servers** (Google 8.8.8.8, Cloudflare 1.1.1.1)
- Tests DNS failover (primary → backup DNS)
- Tests various DNS record types (A, AAAA, MX, CNAME)
- Tests DNS reply formatting and TTL

### Test Coverage
**17 Scheduler Tests:**
- Blocking logic during/outside schedules
- Multiple domains and time slots
- All 7 weekdays
- Warning notification timing
- Edge cases (exact start/end times)

**16 DNS Proxy Tests:**
- Domain blocking and forwarding
- Upstream DNS queries
- Failover behavior
- Record type handling
- TTL and reply flag validation

### Why These Tests Work Without Privileges
- ✅ **Testable pure functions**: Core logic extracted to accept time/config as parameters
- ✅ **Real upstream DNS**: Tests query actual DNS servers to verify forwarding works
- ✅ **No port binding**: DNS logic tested without binding to port 53
- ✅ **No system modifications**: No DNS cache flushing or system DNS changes
- ✅ **No mocking**: Tests use real DNS responses for authenticity

### What's NOT Tested (Requires Privileges)
These features require root/admin and service installation, so they're validated manually:
- ❌ Port 53 binding
- ❌ System DNS cache flushing
- ❌ Browser tab closing
- ❌ Service installation/management

---

## 🧪 Interactive Testing with --test-query

Test whether specific domains would be blocked at any given time **without installing the service or requiring privileges**.

### Quick Test
Check if a domain is blocked at a specific time:
``` bash
./distractions-free --test-query "2024-04-01 10:30" youtube.com
```

### Output Example
```
============================================================
Test Query Result
============================================================
Time:          2024-04-01 10:30 (Monday)
Domain:        youtube.com
------------------------------------------------------------
Status:        🚫 BLOCKED
Response:      0.0.0.0 (blocking response)
------------------------------------------------------------
Applicable Rules:
  Domain: youtube.com
    ✓ Blocked on Monday from 09:00 to 17:00 (ACTIVE)
------------------------------------------------------------
============================================================
```

### Time Format
Use **`2006-01-02 15:04`** format (YYYY-MM-DD HH:MM in 24-hour time):
- Example: `2024-04-01 10:30` = Monday, April 1, 2024 at 10:30 AM
- Example: `2024-04-01 09:00` = Monday, April 1, 2024 at 9:00 AM (start of block)

### Test Cases

**Check if a domain is allowed:**
``` bash
./distractions-free --test-query "2024-04-06 10:30" youtube.com
```
Output: `✓ ALLOWED (forwarded to upstream DNS)` with DNS response

**Test warning trigger (3 minutes before block):**
``` bash
./distractions-free --test-query "2024-04-01 08:57" youtube.com
```
Output: Shows `⚠️ Warning will trigger 3 minutes before block!`

**Multi-slot schedule tests:**
``` bash
./distractions-free --test-query "2024-04-01 10:30" facebook.com   # Monday first block
./distractions-free --test-query "2024-04-01 12:30" facebook.com   # Monday gap between blocks
./distractions-free --test-query "2024-04-01 14:30" facebook.com   # Monday second block
./distractions-free --test-query "2024-04-01 17:45" linkedin.com   # Monday evening block
./distractions-free --test-query "2024-04-02 12:30" linkedin.com   # Tuesday midday block
```

**Test different domains and times:**
``` bash
./distractions-free --test-query "2024-04-01 17:00" facebook.com  # After block ends
./distractions-free --test-query "2024-04-02 10:30" linkedin.com  # Tuesday schedule
```

### Why This Feature is Useful
- ✅ **Verify your rules**: Confirm blocking schedules work as expected
- ✅ **No system impact**: Uses local config file, no service needed
- ✅ **Real DNS queries**: Shows actual upstream DNS responses
- ✅ **Debug schedules**: Check if a specific time/domain/day combination triggers blocking
- ⚠️ **Root still required**: `--no-service` avoids service installation but still binds port 53, which requires root on all Unix systems. Use `sudo` even in this mode.


---

## 🌐 Web UI Test Mode (`}--test-web`)

A beautiful, interactive web interface for testing DNS blocking queries in your browser.

### Launch the Test UI
```bash
./distractions-free --test-web
```
Then open your browser to **http://localhost:8040**

### Features
- **Beautiful gradient UI**: Modern purple gradient design with responsive layout
- **Time input**: Pick any date/time in format `YYYY-MM-DD HH:MM`
- **Domain input**: Enter the domain to test (with example placeholder)
- **Live config viewer**: See the current config in read-only JSON format
- **Real-time results**: Displays:
  - Weekday (calculated from the date)
  - Blocking status with emoji (🚫 BLOCKED or ✓ ALLOWED)
  - Actual DNS response (real IPs from upstream DNS)
  - Applicable rules with schedule details
  - Warning notifications (3-minute pre-block alerts)
- **Color-coded display**: 
  - Red for blocked status
  - Green for allowed status
  - Yellow warnings for pre-block notifications
- **Loading animation**: Spinner while query executes

### Web UI Screenshots
When testing YouTube on Monday at 10:30 (blocked):
- Status shows: 🚫 **BLOCKED**
- DNS Response: `0.0.0.0 (blocking response)`
- Applicable Rules: "✓ ACTIVE: Monday 09:00-17:00"

When testing Google on Saturday at 10:30 (allowed):
- Status shows: ✓ **ALLOWED (forwarded to upstream DNS)**
- DNS Response: `google.com. 287 IN A 142.251.215.110`
- Applicable Rules: "No rules apply"

### Why Use the Web UI Instead of CLI
- 🎨 **Visual**: See results in a beautiful, easy-to-read format
- 🔄 **Fast iterations**: Quickly test multiple times/domains without typing commands
- 📋 **Config viewer**: See your rules configuration side-by-side with results
- 🌟 **Better UX**: Emoji status indicators, color coding, and smooth animations
- ⌨️ **Press Enter to submit**: Type time/domain and hit Enter to test

### Example Usage
1. Launch: `./distractions-free --test-web`
2. Browser opens to http://localhost:8040
3. Time field pre-filled with current time
4. Enter domain: `youtube.com`
5. Click "Test Query" or press Enter
6. Instantly see if it's blocked with rule details
7. Modify time to test different schedules
8. See DNS responses from real upstream servers

---

## 🖥️ AppleScript Test Mode (`--test-applescript`)

Test the macOS AppleScript integration for warning notifications and tab closing **without requiring privileges or service installation**.

### Quick Test
Run the AppleScript test to verify GUI interactions:
``` bash
./distractions-free --test-applescript
```

### What It Does
- **Scans Browser Tabs**: Checks Chrome and Safari for open tabs matching test domains
- **Conditional Warnings**: Shows native macOS alert only for domains that have open tabs
- **Closes Tabs**: Automatically closes tabs for facebook.com after a brief delay
- **Interactive Confirmation**: Asks for user confirmation before executing scripts

### Test Domains
- **Warning Domains**: reddit.com, roblox.com (alert shown only if tabs are open)
- **Close Domains**: facebook.com (tabs closed regardless of open status)

### Output Example
```
=== CLOSE TABS SCRIPT (facebook.com) ===
tell application "Google Chrome"
    repeat with w in windows
        set tabList to tabs of w whose URL contains "facebook.com"
        repeat with t in tabList
            close t
        end repeat
    end repeat
end tell
tell application "Safari"
    repeat with w in windows
        set tabList to tabs of w whose URL contains "facebook.com"
        repeat with t in tabList
            close t
        end repeat
    end repeat
end tell

Execute scripts? (y/N): y
Executing warning script...
Sleeping 2 seconds before close tabs script...
Executing close tabs script...
```

### Why This Feature is Useful
- ✅ **Verify AppleScript**: Test that macOS GUI interactions work correctly
- ✅ **No system impact**: Uses test domains and requires user confirmation
- ✅ **Debug notifications**: Ensure alerts display properly in user context
- ✅ **Test tab closing**: Validate browser automation works across Chrome/Safari
- ✅ **Service compatibility**: Confirms scripts work in both interactive and service modes

---

## 📦 Release Process (For Maintainers)

### How Releases Work

This project uses **GitHub Actions** to automatically build and release binaries whenever a new version tag is pushed.

### Making a Release

1. **Create a version tag** (semantic versioning):
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Automatic Build Process**:
   - GitHub Actions workflow triggers automatically
   - Builds binaries for macOS ARM64 and Windows (x86_64)
   - Runs full test suite on each platform
   - Uploads binaries to the release

3. **Release is Created**:
   - Visit [Releases page](https://github.com/vsangava/distractions-free/releases)
   - Add release notes describing changes
   - Users can now download binaries

### Workflow Details

The `.github/workflows/release.yml` file:
- **Triggers on**: Git tags matching `v*` (e.g., `v1.0.0`)
- **Builds for**:
  - macOS ARM64 (Apple Silicon: M1/M2/M3)
  - Windows x86_64
- **Tests**: Runs `go test ./...` on each platform before release
- **Uploads**: Uses GitHub's release artifacts API

### Example Release Workflow

```bash
# Make final commits and tag version
git tag v1.0.0
git push origin v1.0.0

# → GitHub Actions automatically:
#   1. Builds 3 binaries
#   2. Runs all tests
#   3. Creates release with binaries attached
#   4. Available at https://github.com/vsangava/distractions-free/releases/tag/v1.0.0
```

### Next Steps for Production

When ready for wider distribution:
- [ ] Add code signing for macOS (required for newer macOS versions)
- [ ] Create Homebrew formula for `brew install distractions-free`
- [ ] Create Scoop manifest for Windows package manager
- [ ] Consider Windows installer (.msi) using WiX or NSIS

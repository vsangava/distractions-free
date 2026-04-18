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

---

## 🛠 Prerequisites

If you are developing or compiling this on macOS, you need the Go compiler installed:
``` bash
brew install go
```

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

## 🚀 Build & Installation

### 1. Compile the Binary
Clone the repository and build the self-contained executable:
``` bash
go build -o distractions-free ./cmd/app
```

### 2. Install the Background Service
Because this app runs a local DNS server on port 53, it requires Root/Administrator privileges.
``` bash
sudo ./distractions-free install
```

### 3. Start the Service
``` bash
sudo ./distractions-free start
```

### 4. Point Your OS to the Proxy
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
  "rules":[
    {
      "domain": "youtube.com",
      "is_active": true,
      "schedules": {
        "Monday":[{"start": "09:00", "end": "17:00"}],
        "Tuesday":[{"start": "09:00", "end": "17:00"}]
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

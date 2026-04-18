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

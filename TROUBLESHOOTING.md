# Privileged Operations Testing & Troubleshooting Guide

## Part 1: Testing Privileged Operations

### ✅ What Can Be Tested WITHOUT Service Installation

The current setup allows testing these operations without privileges:

1. **Rule Evaluation Logic** (✓ Verified)
   ```bash
   ./distractions-free --test-query "2024-04-01 10:30" youtube.com
   ```
   - Tests `scheduler.EvaluateRulesAtTime()` (pure function)
   - Queries real upstream DNS
   - Shows what WOULD happen at that time

2. **DNS Response Generation** (✓ Verified)
   - Uses real 8.8.8.8 and 1.1.1.1 DNS servers
   - No port binding needed
   - Tests blocking logic without port 53

3. **Web API** (✓ Verified)
   ```bash
   ./distractions-free --test-web
   curl http://127.0.0.1:8040/api/test-query?time=2024-04-01%2010:30&domain=youtube.com
   ```

4. **Configuration Loading** (✓ Verified)
   - Uses `./config.json` with `UseLocalConfig=true`
   - Tests config parsing without system paths

---

## Part 2: Testing Privileged Operations (Requires sudo)

To test the actual privileged operations, you need to install the service:

### 2.1 Build the Binary

```bash
cd /Users/phani/code/distractions-free
go build -o distractions-free ./cmd/app
```

### 2.2 Set Up Test Config

```bash
# Create the system config directory
sudo mkdir -p "/Library/Application Support/DistractionsFree"

# Create a test config file
sudo tee "/Library/Application Support/DistractionsFree/config.json" > /dev/null << 'EOF'
{
  "settings": {
    "primary_dns": "8.8.8.8:53",
    "backup_dns": "1.1.1.1:53"
  },
  "rules": [
    {
      "domain": "youtube.com",
      "is_active": true,
      "schedules": {
        "Monday": [{"start": "09:00", "end": "17:00"}],
        "Tuesday": [{"start": "09:00", "end": "17:00"}],
        "Wednesday": [{"start": "09:00", "end": "17:00"}],
        "Thursday": [{"start": "09:00", "end": "17:00"}],
        "Friday": [{"start": "09:00", "end": "17:00"}]
      }
    }
  ]
}
EOF

echo "Config file created successfully"
```

### 2.3 Install the Service

```bash
sudo ./distractions-free install
```

**Expected output**:
```
[sudo] password for phani:
Installing service... 
Service installed successfully
```

**Verify installation**:
```bash
# Check if LaunchAgent is created
ls -la ~/Library/LaunchAgents/com.github.distractions-free.plist
```

If successful, you should see:
```
-rw-r--r-- 1 phani staff 1234 Apr 18 10:00 ~/Library/LaunchAgents/com.github.distractions-free.plist
```

### 2.4 Start the Service

```bash
sudo ./distractions-free start
```

**Expected output**:
```
Service started successfully
```

**Verify it's running**:
```bash
sudo ./distractions-free status
```

Should show:
```
Service is running
```

### 2.5 Test Privileged Operations

#### Test 1: Port 53 Binding

```bash
# Check if DNS server is listening on port 53
sudo lsof -i :53

# Expected output (should show distractions-free process)
```

#### Test 2: System DNS Configuration

```bash
# Check current DNS settings
networksetup -getdnsservers Wi-Fi

# Expected output AFTER service starts should show:
# 127.0.0.1 (or similar if configured)

# View all DNS configurations
scutil --dns | grep -A 5 "resolver"
```

#### Test 3: DNS Resolution Through Proxy

```bash
# Query through system DNS (which is now pointing to our proxy)
nslookup youtube.com 127.0.0.1

# If youtube.com is blocked at current time, you should get:
# Address: 0.0.0.0

# If it's allowed (or outside schedule), you should get:
# Address: actual IP address
```

#### Test 4: Web Dashboard Access

```bash
# Access the dashboard
open http://127.0.0.1:8040

# Should see:
# - Dashboard page
# - Current config JSON
# - Test query interface (still works in service mode)
```

### 2.6 Stop the Service

```bash
sudo ./distractions-free stop
```

**Expected output**:
```
Service stopped successfully
```

**Verify DNS is restored**:
```bash
# Check DNS settings are restored to system defaults
networksetup -getdnsservers Wi-Fi

# Should show original DNS servers (not 127.0.0.1)
```

### 2.7 Uninstall the Service

```bash
sudo ./distractions-free uninstall
```

**Expected output**:
```
Service uninstalled successfully
```

**Verify uninstallation**:
```bash
# Check if LaunchAgent is removed
ls -la ~/Library/LaunchAgents/com.github.distractions-free.plist

# Should show: No such file or directory
```

---

## Part 3: Debugging - Viewing Logs

### 3.1 LaunchAgent Logs (macOS)

When the service is installed as a LaunchAgent, logs go to standard output/error files:

```bash
# View service logs
log stream --predicate 'process == "distractions-free"' --level debug

# Or view system logs for the service
log show --predicate 'process == "distractions-free"' --last 1h

# View last 100 lines in real-time
tail -f /var/log/system.log | grep "distractions-free"
```

### 3.2 LaunchAgent Plist File

```bash
# View the actual LaunchAgent configuration
cat ~/Library/LaunchAgents/com.github.distractions-free.plist

# Expected content (XML format):
# <key>Label</key>
# <string>com.github.distractions-free</string>
# <key>Program</key>
# <string>/path/to/distractions-free</string>
# <key>RunAtLoad</key>
# <true/>
```

### 3.3 Service Status Commands

```bash
# Check if service is loaded
launchctl list | grep distractions-free

# Check service logs through launchctl
launchctl log level all
launchctl log show system | grep distractions-free

# Get detailed service information
launchctl print system/com.github.distractions-free
```

### 3.4 Process Monitoring

```bash
# Check if process is running
ps aux | grep distractions-free

# Expected (if running):
# root    12345   0.0  0.1 234567 8901 ??  Ss   10:00AM   0:00.25 /path/to/distractions-free

# Check process status with detailed info
sudo lsof -c distractions-free

# Should show:
# - Open files/directories
# - Network connections (port 53)
# - Memory usage
```

### 3.5 DNS Server Verification

```bash
# Check if DNS server is bound to port 53
sudo lsof -i :53

# Expected output:
# COMMAND    PID USER   FD   TYPE  DEVICE SIZE/OFF NODE NAME
# distractions-free 12345 root    3u  IPv4  0x1234567      0t0  UDP 127.0.0.1:53

# Check DNS traffic
sudo tcpdump -i lo0 port 53

# Should show DNS queries passing through port 53
```

### 3.6 System DNS Configuration Log

```bash
# View system DNS resolver configuration
scutil --dns

# Should show:
# resolver #1
#   search domain[0] : local
#   nameserver[0] : 127.0.0.1  (if our proxy is set as primary)
```

---

## Part 4: Common Troubleshooting Issues

### Issue 1: Service Won't Start - "Permission Denied"

**Symptom**:
```
Error: listen tcp 127.0.0.1:53: permission denied
```

**Cause**: Port 53 binding requires root privileges

**Solution**:
```bash
# Make sure you're using sudo
sudo ./distractions-free start

# Check if another process is using port 53
sudo lsof -i :53

# If something else is using it, kill it first
sudo kill -9 <PID>
```

### Issue 2: Installation Fails - "LaunchAgent Already Exists"

**Symptom**:
```
Error: service is already installed
```

**Solution**:
```bash
# Uninstall first
sudo ./distractions-free uninstall

# Or manually remove the plist
rm ~/Library/LaunchAgents/com.github.distractions-free.plist

# Then install again
sudo ./distractions-free install
```

### Issue 3: DNS Not Being Intercepted

**Symptom**: Blocked domains still load

**Troubleshooting**:
```bash
# 1. Verify service is running
sudo ./distractions-free status

# 2. Check if DNS is pointed to our proxy
networksetup -getdnsservers Wi-Fi

# 3. Check if blocked domains are in config
cat "/Library/Application Support/DistractionsFree/config.json"

# 4. Check current time matches block schedule
date  # Compare with config schedule

# 5. Query DNS directly to our proxy
dig @127.0.0.1 youtube.com

# 6. Check if port 53 is listening
sudo lsof -i :53
```

### Issue 4: Service Crashes - "Segmentation Fault"

**Symptom**:
```
Error: service exited with status 139 (segmentation fault)
```

**Troubleshooting**:
```bash
# 1. Check system logs for crash details
log show --predicate 'process == "distractions-free"' --last 1h

# 2. Check if config file is valid JSON
cat "/Library/Application Support/DistractionsFree/config.json" | python3 -m json.tool

# 3. Try running in no-service mode for debugging
./distractions-free --no-service

# 4. Check for OS-specific issues
uname -a
go version
```

### Issue 5: Can't Uninstall - "Service Still Running"

**Symptom**:
```
Error: cannot uninstall while service is running
```

**Solution**:
```bash
# Stop the service first
sudo ./distractions-free stop

# Wait a moment
sleep 2

# Then uninstall
sudo ./distractions-free uninstall

# If still stuck, forcefully stop it
sudo pkill -f "distractions-free"

# Remove LaunchAgent manually
rm ~/Library/LaunchAgents/com.github.distractions-free.plist
```

---

## Part 5: Verification Checklist

Use this checklist to verify all privileged operations are working:

```
Installation & Registration:
  [ ] Binary builds without errors
  [ ] Config file created at /Library/Application Support/DistractionsFree/config.json
  [ ] LaunchAgent plist exists at ~/Library/LaunchAgents/com.github.distractions-free.plist
  [ ] Service appears in launchctl list

Service Startup:
  [ ] Service starts without errors
  [ ] Process appears in ps aux
  [ ] No "Permission denied" errors
  [ ] Service status shows "running"

Port 53 Binding:
  [ ] Port 53 is listening (lsof -i :53)
  [ ] Only our process is bound to port 53
  [ ] Can send DNS queries to 127.0.0.1:53

System DNS Configuration:
  [ ] networksetup shows 127.0.0.1 as nameserver
  [ ] DNS cache flush completes
  [ ] mDNSResponder is active

DNS Interception:
  [ ] Blocked domain returns 0.0.0.0
  [ ] Allowed domain returns real IP
  [ ] DNS failover works (primary → backup)

Web Dashboard:
  [ ] Dashboard accessible at 127.0.0.1:8040
  [ ] Config API returns valid JSON
  [ ] Test query API works

Service Cleanup:
  [ ] Service stops cleanly
  [ ] DNS is restored to original
  [ ] networksetup shows original DNS servers
  [ ] Uninstallation completes
  [ ] LaunchAgent plist is removed
```

---

## Part 6: Advanced Debugging

### Real-Time Log Monitoring

```bash
# Monitor all DNS queries in real-time
sudo tcpdump -i lo0 -nn port 53

# Monitor service logs with timestamps
sudo log stream --predicate 'process == "distractions-free"' --level debug --style compact

# Monitor system events
log stream --predicate 'eventMessage contains "distractions"' --level debug
```

### Performance Profiling

```bash
# Profile CPU usage
sudo sample distractions-free 5

# Check memory usage
ps aux | grep distractions-free | awk '{print $2, $3, $4, $6}'

# Monitor network connections
netstat -an | grep 53
```

### Config Reload Testing

```bash
# 1. Start service
sudo ./distractions-free start

# 2. Query a domain (should be allowed)
dig @127.0.0.1 example.com

# 3. Edit config to block example.com
sudo vi "/Library/Application Support/DistractionsFree/config.json"

# 4. Wait up to 1 minute for scheduler to reload

# 5. Query again (should now be blocked or allowed based on new config)
dig @127.0.0.1 example.com

# Check logs to see when config was reloaded
log show --predicate 'process == "distractions-free"' --last 2m
```


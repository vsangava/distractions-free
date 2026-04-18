# Distractions-Free: System Design Document

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Core Modules](#core-modules)
4. [Data Flow](#data-flow)
5. [Service Operation](#service-operation)
6. [Privileged Operations](#privileged-operations)
7. [Testing Architecture](#testing-architecture)
8. [Configuration](#configuration)
9. [Mode of Operation](#mode-of-operation)

---

## Overview

**Distractions-Free** is a system-level DNS proxy that enforces productivity schedules by intercepting DNS requests. Instead of allowing users to bypass time-blocking with browser extensions, this application runs as a background service with privileged access, making it impossible to disable without root password.

### Key Characteristics
- **System-level enforcement**: Runs as a service (macOS `launchd`, Windows `Service`)
- **Zero-trust design**: Works even if applications try to bypass the OS DNS
- **Real-time scheduling**: Evaluates rules every minute to handle laptop sleep/wake
- **Multi-layer testing**: Comprehensive unit tests without requiring privileges or port binding
- **Interactive testing**: CLI and web UI for testing blocking logic without service installation

### Technology Stack
- **Language**: Go 1.x
- **DNS Library**: github.com/miekg/dns
- **Service Framework**: github.com/kardianos/service
- **Port**: 127.0.0.1:53 (requires root/admin)
- **Web Server**: net/http (embedded, no external dependencies)

---

## Architecture

### System-Level Components Diagram
```
┌─────────────────────────────────────────────────────────────────┐
│                      Operating System                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  System DNS: 127.0.0.1:53 (configured via networksetup)│    │
│  └────────────────────────┬────────────────────────────────┘    │
│                           │                                      │
│                           ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │      Distractions-Free Service (running as root)        │    │
│  │                                                         │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │    │
│  │  │  Scheduler   │  │   Config     │  │  Web Server  │ │    │
│  │  │  (1-min)     │  │  (Config.json)│ │ (8040)      │ │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │    │
│  │                           ▲                                   │
│  │                           │                                   │
│  │              ┌────────────┴────────────┐                     │
│  │              ▼                         ▼                     │
│  │   ┌──────────────────┐      ┌──────────────────┐             │
│  │   │  DNS Proxy       │      │  Active Blocks   │             │
│  │   │  (Port 53)       │      │  (Map)           │             │
│  │   └──────────────────┘      └──────────────────┘             │
│  │              │                                               │
│  │              ▼                                               │
│  │   ┌──────────────────┐      ┌──────────────────┐             │
│  │   │  Upstream DNS    │      │  Tab Closer      │             │
│  │   │  (8.8.8.8:53)    │      │  (AppleScript)   │             │
│  │   └──────────────────┘      └──────────────────┘             │
│  │                                                              │
│  └──────────────────────────────────────────────────────────────┘
│                                                                   │
└─────────────────────────────────────────────────────────────────┘

  Application Requests
        │
        ▼
  What IP is youtube.com?
        │
        ▼
  ┌─────────────────────────────────────────┐
  │  Is youtube.com in blocked domains?      │
  │  (Check current time and rules)          │
  └─────────────────────────────────────────┘
        │
        ├─ YES ──► Return 0.0.0.0 (BLOCKED)
        │
        └─ NO  ──► Forward to upstream DNS
                   (Google 8.8.8.8 or Cloudflare)
```

### Module Dependencies
```
cmd/app/main.go
│
├── config
│   └── config.go (loads/stores config.json)
│
├── scheduler
│   └── scheduler.go (evaluates rules every minute)
│       ├── config
│       └── proxy (updates blocked domains)
│
├── proxy
│   └── dns.go (intercepts port 53 DNS requests)
│       ├── config (gets primary/backup DNS)
│       └── miekg/dns (DNS protocol handling)
│
├── web
│   ├── server.go (HTTP dashboard at :8040)
│   └── static/* (embedded HTML/CSS/JS)
│
└── testcli
    └── testcli.go (testing without service)
        ├── config
        ├── scheduler
        └── proxy
```

---

## Core Modules

### 1. Config Module (`internal/config/config.go`)

**Purpose**: Centralized configuration management with thread-safe access.

**Key Structures**:
```go
type Config struct {
    Settings Settings
    Rules    []Rule
}

type Rule struct {
    Domain    string                // e.g., "youtube.com"
    IsActive  bool                  // Can disable/enable rules
    Schedules map[string][]TimeSlot // Key: "Monday", "Tuesday", etc.
}

type TimeSlot struct {
    Start string // "09:00"
    End   string // "17:00"
}

type Settings struct {
    PrimaryDNS string // e.g., "8.8.8.8:53"
    BackupDNS  string // e.g., "1.1.1.1:53"
}
```

**Config File Paths**:
- **macOS**: `/Library/Application Support/DistractionsFree/config.json`
- **Windows**: `C:\ProgramData\DistractionsFree\config.json`
- **Linux**: `/etc/distractionsfree/config.json`
- **Test Mode** (`UseLocalConfig=true`): `./config.json` (current directory)

**Key Functions**:

1. **GetConfigFilePath()**
   - Returns the OS-appropriate config path
   - Creates the directory if it doesn't exist
   - Uses `UseLocalConfig` flag for test mode

   ```go
   func GetConfigFilePath() (string, error) {
       if UseLocalConfig {
           return "." // Current directory for testing
       }
       // Platform-specific paths...
   }
   ```

2. **LoadConfig()**
   - Reads `config.json` from disk
   - Parses JSON into `AppConfig`
   - Returns error if file doesn't exist or is malformed

   ```go
   func LoadConfig() error {
       filePath, err := GetConfigFilePath()
       // ... read file, parse JSON
       mu.Lock()
       AppConfig = cfg
       mu.Unlock()
       return nil
   }
   ```

3. **GetConfig()**
   - Returns a copy of current config
   - Thread-safe using RWMutex
   - Used by all modules that need access to rules

   ```go
   func GetConfig() Config {
       mu.RLock()
       defer mu.RUnlock()
       return AppConfig
   }
   ```

**Thread Safety**:
- Uses `sync.RWMutex` for concurrent reads (many goroutines can read simultaneously)
- Exclusive write lock only when loading new config

---

### 2. Scheduler Module (`internal/scheduler/scheduler.go`)

**Purpose**: Evaluates blocking rules at each minute and manages state transitions (tab closing, warnings).

**Key Concept**: The scheduler uses a **testable pure function** (`EvaluateRulesAtTime`) that accepts time as a parameter, making it possible to unit test without mocking time.

**Key Functions**:

1. **EvaluateRulesAtTime(t time.Time, cfg config.Config) map[string]bool**

   This is the **core blocking logic** - it's a pure function with no side effects.

   ```go
   func EvaluateRulesAtTime(t time.Time, cfg config.Config) map[string]bool {
       currentDay := t.Weekday().String()   // "Monday", "Tuesday", etc.
       currentTime := t.Format("15:04")     // "10:30", "14:45", etc.
       
       newBlocked := make(map[string]bool)
       
       for _, rule := range cfg.Rules {
           if !rule.IsActive {
               continue
           }
           if slots, exists := rule.Schedules[currentDay]; exists {
               for _, slot := range slots {
                   // Check if current time falls within the slot
                   if currentTime >= slot.Start && currentTime < slot.End {
                       newBlocked[rule.Domain] = true
                       break
                   }
               }
           }
       }
       
       return newBlocked
   }
   ```

   **Example**: Testing if youtube.com is blocked on Monday 10:30:
   - `currentDay = "Monday"`
   - `currentTime = "10:30"`
   - Find youtube.com rule with schedule: `"Monday": [{"Start": "09:00", "End": "17:00"}]`
   - Check: `"10:30" >= "09:00" && "10:30" < "17:00"` → **TRUE** → youtube.com is blocked

2. **CheckWarningDomainsAtTime(t time.Time, cfg config.Config) []string**

   Returns domains that should trigger 3-minute warnings at this time.

   ```go
   func CheckWarningDomainsAtTime(t time.Time, cfg config.Config) []string {
       currentDay := t.Weekday().String()
       futureTime := t.Add(3 * time.Minute).Format("15:04")
       
       var warningDomains []string
       
       for _, rule := range cfg.Rules {
           if !rule.IsActive {
               continue
           }
           if slots, exists := rule.Schedules[currentDay]; exists {
               for _, slot := range slots {
                   // Warning triggers exactly 3 minutes before block
                   if futureTime == slot.Start {
                       warningDomains = append(warningDomains, rule.Domain)
                   }
               }
           }
       }
       
       return warningDomains
   }
   ```

   **Example**: At 08:57 AM on Monday, YouTube block starts at 09:00:
   - `futureTime = 08:57 + 3 min = 09:00`
   - Find youtube.com with start time 09:00
   - `09:00 == 09:00` → **TRUE** → Include in warnings

3. **Start()**

   Launches the scheduler loop that runs every minute.

   ```go
   func Start() {
       ticker := time.NewTicker(1 * time.Minute)
       go func() {
           evaluateRules()  // Run immediately, don't wait 1 minute
           for range ticker.C {
               evaluateRules()  // Then run every minute
           }
       }()
   }
   ```

4. **evaluateRules() (internal)**

   Called every minute - orchestrates the rule evaluation, state transitions, and side effects.

   ```go
   func evaluateRules() {
       cfg := config.GetConfig()
       
       // Get blocked domains at this exact moment
       newBlocked := EvaluateRulesAtTime(time.Now(), cfg)
       
       // Detect transitions
       for domain, wasBlocked := range activeBlocks {
           isNowBlocked := newBlocked[domain]
           if wasBlocked && !isNowBlocked {
               // TRANSITION: Blocked → Allowed (block ended)
               log.Printf("Block ended for %s, reopening tabs", domain)
               if runtime.GOOS == "darwin" {
                   reopenTabs(domain)  // AppleScript to reopen closed tabs
               }
           }
       }
       
       // Detect warnings
       warnings := CheckWarningDomainsAtTime(time.Now(), cfg)
       for _, domain := range warnings {
           log.Printf("3-minute warning for %s", domain)
           sendNotification(domain)  // Native notification
       }
       
       // Update proxy with new blocked domains
       proxy.UpdateBlockedDomains(newBlocked)
       
       // Store for next iteration
       activeBlocks = newBlocked
   }
   ```

**State Transitions**:
```
Time: 08:57 on Monday
  ├─ Check warnings: youtube.com block starts in 3 min
  ├─ Send notification: "YouTube will be blocked in 3 minutes"
  └─ User sees warning

Time: 09:00 on Monday
  ├─ Check blocking: youtube.com now active
  ├─ Update proxy's blocked map
  ├─ AppleScript closes YouTube tabs in Chrome/Safari
  └─ youtube.com DNS requests return 0.0.0.0

Time: 17:00 on Monday
  ├─ Check blocking: youtube.com block ended
  ├─ Update proxy's blocked map
  └─ youtube.com DNS requests forward to upstream DNS
```

---

### 3. DNS Proxy Module (`internal/proxy/dns.go`)

**Purpose**: Intercepts DNS requests on port 53 and either blocks or forwards them.

**Key Concept**: Uses a **testable pure function** (`GetDNSResponse`) that processes DNS queries without binding to a port, enabling unit tests with real upstream DNS servers.

**Key Data Structures**:
```go
var (
    blockedDomains map[string]bool  // Fast O(1) lookup
    blockMu        sync.RWMutex     // Thread-safe access
)
```

**Key Functions**:

1. **GetDNSResponse(r *dns.Msg, blockedDomainsList map[string]bool, primaryDNS, backupDNS string) (*dns.Msg, error)**

   The testable DNS query processor.

   ```go
   func GetDNSResponse(r *dns.Msg, blockedDomainsList map[string]bool, 
                       primaryDNS, backupDNS string) (*dns.Msg, error) {
       m := new(dns.Msg)
       m.SetReply(r)  // Copy request ID, question, flags
       m.Compress = false
       
       if len(r.Question) == 0 {
           return m, nil  // No questions, return empty
       }
       
       q := r.Question[0]
       domain := strings.TrimSuffix(q.Name, ".")  // "youtube.com." → "youtube.com"
       
       // Check if domain is blocked
       isBlocked := blockedDomainsList[domain]
       
       if isBlocked && q.Qtype == dns.TypeA {
           // Return 0.0.0.0 for blocked A records
           rr, _ := dns.NewRR(q.Name + " 60 IN A 0.0.0.0")
           m.Answer = append(m.Answer, rr)
           return m, nil
       }
       
       // Forward to upstream DNS
       c := new(dns.Client)
       
       in, _, err := c.Exchange(r, primaryDNS)
       if err != nil {
           // Failover to backup DNS
           in, _, err = c.Exchange(r, backupDNS)
           if err != nil {
               return nil, err
           }
       }
       return in, nil
   }
   ```

   **DNS Response Examples**:

   *Blocked Query*:
   ```
   Query:  Q: youtube.com A?
   Answer: youtube.com 60 IN A 0.0.0.0
   ```

   *Allowed Query*:
   ```
   Query:  Q: google.com A?
   Answer: google.com 287 IN A 142.251.215.110  (from upstream)
   ```

2. **UpdateBlockedDomains(newBlocked map[string]bool)**

   Thread-safe update of blocked domains (called by scheduler every minute).

   ```go
   func UpdateBlockedDomains(newBlocked map[string]bool) {
       blockMu.Lock()
       defer blockMu.Unlock()
       blockedDomains = newBlocked
   }
   ```

3. **handleDNSRequest(w dns.ResponseWriter, r *dns.Msg)**

   The actual DNS request handler (called for each incoming DNS query).

   ```go
   func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
       // Get current blocked list (read lock)
       blockMu.RLock()
       blockedCopy := blockedDomains
       blockMu.RUnlock()
       
       cfg := config.GetConfig()
       
       // Process request
       m, err := GetDNSResponse(r, blockedCopy, 
                                cfg.Settings.PrimaryDNS, 
                                cfg.Settings.BackupDNS)
       if err != nil {
           dns.HandleFailed(w, r)
           return
       }
       
       // Send response
       w.WriteMsg(m)
   }
   ```

4. **StartDNSServer()**

   Binds to port 53 and starts accepting DNS requests. **Requires root/admin privileges.**

   ```go
   func StartDNSServer() {
       UpdateBlockedDomains(make(map[string]bool))
       dns.HandleFunc(".", handleDNSRequest)
       
       server := &dns.Server{Addr: "127.0.0.1:53", Net: "udp"}
       log.Printf("Starting local DNS proxy on 127.0.0.1:53...")
       if err := server.ListenAndServe(); err != nil {
           log.Fatalf("Failed to start DNS server: %s", err.Error())
       }
   }
   ```

**DNS Flow Diagram**:
```
Application: "What's the IP for youtube.com?"
             │
             ▼
System DNS (127.0.0.1:53) ← configured via networksetup
             │
             ▼
handleDNSRequest()
    │
    ├─ Get current blocked domains list
    │
    ├─ Parse DNS query
    │
    └─ Call GetDNSResponse()
           │
           ├─ Is "youtube.com" in blocked list?
           │
           ├─ YES: Return 0.0.0.0
           │       Application tries to connect to 0.0.0.0 → Connection fails
           │
           └─ NO: Forward to upstream DNS (8.8.8.8:53)
                  │
                  └─ Response: "142.251.215.110" (actual IP)
                     Application connects successfully
```

---

### 4. Web Server Module (`internal/web/server.go`)

**Purpose**: Provides HTTP API and dashboard at `http://127.0.0.1:8040`.

**Key Functions**:

1. **StartWebServer()**

   Regular web server (runs alongside DNS proxy when service is active).

   ```go
   func StartWebServer() {
       staticHandler, err := StaticFileHandler()
       if err != nil {
           log.Fatalf("Failed to load embedded web files: %v", err)
       }
       
       http.Handle("/", staticHandler)          // Serve static files
       http.HandleFunc("/api/config", ConfigHandler)
       http.HandleFunc("/api/test-query", TestQueryHandler)
       
       log.Println("Web server starting on http://127.0.0.1:8040")
       if err := http.ListenAndServe("127.0.0.1:8040", nil); err != nil {
           log.Fatalf("Web server failed: %v", err)
       }
   }
   ```

2. **StartTestWebServer()**

   Dedicated test UI server (runs when `--test-web` flag is used).

   ```go
   func StartTestWebServer() {
       http.HandleFunc("/", TestPageHandler)        // Serve test UI page
       http.HandleFunc("/api/config", ConfigHandler)
       http.HandleFunc("/api/test-query", TestQueryHandler)
       
       log.Println("Test web server starting on http://127.0.0.1:8040")
       if err := http.ListenAndServe("127.0.0.1:8040", nil); err != nil {
           log.Fatalf("Test web server failed: %v", err)
       }
   }
   ```

3. **ConfigHandler(w http.ResponseWriter, r *http.Request)**

   Returns current config as JSON.

   ```go
   func ConfigHandler(w http.ResponseWriter, r *http.Request) {
       w.Header().Set("Content-Type", "application/json")
       cfg := config.GetConfig()
       json.NewEncoder(w).Encode(cfg)
   }
   ```

   Response:
   ```json
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
           "Monday": [{"start": "09:00", "end": "17:00"}]
         }
       }
     ]
   }
   ```

4. **TestQueryHandler(w http.ResponseWriter, r *http.Request)**

   Processes test queries (both CLI and web UI use this).

   ```go
   func TestQueryHandler(w http.ResponseWriter, r *http.Request) {
       w.Header().Set("Content-Type", "application/json")
       
       timeStr := r.URL.Query().Get("time")     // "2024-04-01 10:30"
       domain := r.URL.Query().Get("domain")    // "youtube.com"
       
       if timeStr == "" || domain == "" {
           w.WriteHeader(http.StatusBadRequest)
           json.NewEncoder(w).Encode(map[string]string{
               "error": "Missing time or domain parameter",
           })
           return
       }
       
       result := testcli.GetQueryResult(timeStr, domain)
       
       w.WriteHeader(http.StatusOK)
       json.NewEncoder(w).Encode(result)
   }
   ```

5. **TestPageHandler(w http.ResponseWriter, r *http.Request)**

   Serves the beautiful web UI page with embedded HTML/CSS/JavaScript.

   - Purple gradient background (#667eea → #764ba2)
   - Responsive form with time and domain inputs
   - Live config JSON viewer
   - Real-time results display with color coding
   - Loading spinner animation
   - All UI is embedded in the binary (no external files needed)

---

### 5. Test CLI Module (`internal/testcli/testcli.go`)

**Purpose**: Provides structured testing without service installation or privileges.

**Key Structures**:
```go
type QueryResult struct {
    Time            string     // "2024-04-01 10:30"
    Weekday         string     // "Monday"
    Domain          string     // "youtube.com"
    IsBlocked       bool       // true
    BlockingStatus  string     // "🚫 BLOCKED"
    DNSResponse     string     // "0.0.0.0 (blocking response)"
    ApplicableRules []RuleInfo // Rules that apply
    HasWarning      bool       // Will warn in 3 mins?
    WarningMessage  string     // "⚠️ Warning will trigger..."
    Error           string     // Error if any
}

type RuleInfo struct {
    Domain    string
    Schedules []ScheduleInfo
}

type ScheduleInfo struct {
    Weekday  string // "Monday"
    Start    string // "09:00"
    End      string // "17:00"
    IsActive bool   // Is active right now?
}
```

**Key Functions**:

1. **GetQueryResult(timeStr, domain string) QueryResult**

   Returns structured test result (used by both CLI and web UI).

   ```go
   func GetQueryResult(timeStr, domain string) QueryResult {
       result := QueryResult{
           Time:   timeStr,
           Domain: domain,
       }
       
       // Parse time: "2024-04-01 10:30"
       testTime, err := time.Parse(timeFormat, timeStr)
       if err != nil {
           result.Error = fmt.Sprintf("Invalid time format. Use: %s", timeFormat)
           return result
       }
       
       result.Weekday = testTime.Weekday().String()
       
       // Normalize domain
       domain = strings.TrimSuffix(domain, ".")
       if domain == "" {
           result.Error = "Domain cannot be empty"
           return result
       }
       
       // Load config
       if err := config.LoadConfig(); err != nil {
           result.Error = fmt.Sprintf("Failed to load config: %v", err)
           return result
       }
       
       cfg := config.GetConfig()
       
       // Evaluate blocking rules at this time
       blockedDomains := scheduler.EvaluateRulesAtTime(testTime, cfg)
       result.IsBlocked = blockedDomains[domain]
       
       // Create DNS query
       dnsQuery := new(dns.Msg)
       dnsQuery.SetQuestion(domain+".", dns.TypeA)
       
       // Get DNS response
       response, err := proxy.GetDNSResponse(dnsQuery, blockedDomains,
                                             cfg.Settings.PrimaryDNS,
                                             cfg.Settings.BackupDNS)
       if err != nil {
           result.Error = fmt.Sprintf("DNS query failed: %v", err)
           return result
       }
       
       // Format result
       if result.IsBlocked {
           result.BlockingStatus = "🚫 BLOCKED"
           if len(response.Answer) > 0 {
               if a, ok := response.Answer[0].(*dns.A); ok {
                   result.DNSResponse = fmt.Sprintf("%s (blocking response)", a.A.String())
               }
           }
       } else {
           result.BlockingStatus = "✓ ALLOWED (forwarded to upstream DNS)"
           if len(response.Answer) > 0 {
               result.DNSResponse = response.Answer[0].String()
           }
       }
       
       // Find applicable rules
       for _, rule := range cfg.Rules {
           if !rule.IsActive || rule.Domain != domain {
               continue
           }
           
           ruleInfo := RuleInfo{Domain: rule.Domain}
           
           if slots, exists := rule.Schedules[testTime.Weekday().String()]; exists {
               for _, slot := range slots {
                   currentTime := testTime.Format("15:04")
                   isActive := currentTime >= slot.Start && currentTime < slot.End
                   ruleInfo.Schedules = append(ruleInfo.Schedules, ScheduleInfo{
                       Weekday:  testTime.Weekday().String(),
                       Start:    slot.Start,
                       End:      slot.End,
                       IsActive: isActive,
                   })
               }
           }
           
           if len(ruleInfo.Schedules) > 0 {
               result.ApplicableRules = append(result.ApplicableRules, ruleInfo)
           }
       }
       
       // Check for warnings
       warnings := scheduler.CheckWarningDomainsAtTime(testTime, cfg)
       if contains(warnings, domain) {
           result.HasWarning = true
           result.WarningMessage = "⚠️ Warning will trigger 3 minutes before block!"
       }
       
       return result
   }
   ```

2. **QueryBlocking(timeStr, domain string) error**

   CLI version - prints formatted output to stdout.

   ```go
   func QueryBlocking(timeStr, domain string) error {
       result := GetQueryResult(timeStr, domain)
       
       if result.Error != "" {
           return fmt.Errorf(result.Error)
       }
       
       // Print formatted output
       separator := strings.Repeat("=", 60)
       dashLine := strings.Repeat("-", 60)
       
       fmt.Println(separator)
       fmt.Printf("Test Query Result\n")
       fmt.Println(separator)
       fmt.Printf("Time:          %s (%s)\n", result.Time, result.Weekday)
       fmt.Printf("Domain:        %s\n", result.Domain)
       fmt.Println(dashLine)
       fmt.Printf("Status:        %s\n", result.BlockingStatus)
       fmt.Printf("Response:      %s\n", result.DNSResponse)
       // ... print rules, warnings, etc.
       fmt.Println(separator)
       
       return nil
   }
   ```

---

## Data Flow

### Flow 1: Service Startup (`./distractions-free install && ./distractions-free start`)

```
main.go
  │
  ├─ Parse args → "start" (service management command)
  │
  ├─ Create program{} struct implementing service.Service
  │
  ├─ Create service with Config {Name: "DistractionsFree", ...}
  │
  ├─ Call service.Control(s, "start")
  │   │
  │   ├─ Service framework registers launchd agent (macOS) or Windows Service
  │   │
  │   └─ Framework calls program.Start(s)
  │        │
  │        ├─ Spawn goroutine: p.run()
  │        │
  │        └─ Return success immediately (background)
  │
  └─ Exit (service continues in background)

Background Service (p.run())
  │
  ├─ config.LoadConfig()
  │  └─ Read /Library/Application Support/DistractionsFree/config.json
  │
  ├─ scheduler.Start()
  │  └─ Start 1-minute ticker that calls evaluateRules()
  │
  ├─ web.StartWebServer()
  │  └─ Bind to 127.0.0.1:8040 (dashboard)
  │
  └─ proxy.StartDNSServer()
     └─ Bind to 127.0.0.1:53 (requires root, blocks until service stops)
```

### Flow 2: DNS Request (10:30 AM on Monday, YouTube blocked 09:00-17:00)

```
1. Application calls getaddrinfo("youtube.com")
   │
   ▼
2. OS looks up system DNS: 127.0.0.1:53
   │
   ▼
3. DNS packet arrives at proxy.StartDNSServer()
   │
   ├─ handleDNSRequest(w, r)
   │  │
   │  ├─ Get current blockedDomains from scheduler
   │  │  └─ {"youtube.com": true, "facebook.com": false}
   │  │
   │  └─ Call proxy.GetDNSResponse(query, blocked, dnsServers...)
   │     │
   │     ├─ Extract domain: "youtube.com."  → "youtube.com"
   │     │
   │     ├─ Check: blockedDomainsList["youtube.com"] = true
   │     │
   │     └─ Return: "youtube.com 60 IN A 0.0.0.0"
   │
   ▼
4. DNS response sent to application
   │
   ▼
5. Application tries: connect("0.0.0.0:443")
   │
   ▼
6. Connection fails → YouTube is blocked ✓
```

### Flow 3: Scheduler Minute Tick (09:00 AM on Monday)

```
Time: 09:00 AM Monday
  │
  ▼
scheduler.Start() ticker fires
  │
  ▼
evaluateRules() called
  │
  ├─ cfg = config.GetConfig()
  │
  ├─ oldBlocked = activeBlocks  (previous minute's blocked domains)
  │
  ├─ newBlocked = scheduler.EvaluateRulesAtTime(now.Time(), cfg)
  │  │
  │  └─ Loop through rules:
  │     - youtube.com: Monday 09:00-17:00
  │       - currentTime = "09:00"
  │       - Check: "09:00" >= "09:00" && "09:00" < "17:00" ✓ → BLOCKED
  │     - facebook.com: Monday 13:00-16:00
  │       - Check: "09:00" >= "13:00" && "09:00" < "16:00" ✗ → ALLOWED
  │
  ├─ Detect state transitions
  │  └─ oldBlocked has "twitter.com" but newBlocked doesn't
  │     └─ Block ended! Close tabs with AppleScript
  │
  ├─ Check warnings (3 minutes before)
  │  └─ CheckWarningDomainsAtTime() returns []
  │
  ├─ proxy.UpdateBlockedDomains(newBlocked)
  │  └─ Update the blocked map for DNS requests
  │
  └─ activeBlocks = newBlocked
     └─ Store for next comparison
```

### Flow 4: Test Query (`./distractions-free --test-query "2024-04-01 10:30" youtube.com`)

```
main.go
  │
  ├─ Parse args → "--test-query", "2024-04-01 10:30", "youtube.com"
  │
  ├─ Set config.UseLocalConfig = true
  │
  └─ testcli.QueryBlocking("2024-04-01 10:30", "youtube.com")
     │
     └─ testcli.GetQueryResult(...)
        │
        ├─ Parse time: "2024-04-01 10:30" → time.Time{...}
        │  └─ Weekday = Monday
        │
        ├─ Load config from ./config.json
        │
        ├─ Call scheduler.EvaluateRulesAtTime(parsedTime, cfg)
        │  └─ Returns {"youtube.com": true, ...}
        │
        ├─ Build DNS query: "youtube.com A?"
        │
        ├─ Call proxy.GetDNSResponse(query, blocked, ...)
        │  └─ Returns "youtube.com. 60 IN A 0.0.0.0"
        │
        └─ Return QueryResult {
              Time: "2024-04-01 10:30",
              Weekday: "Monday",
              Domain: "youtube.com",
              IsBlocked: true,
              BlockingStatus: "🚫 BLOCKED",
              DNSResponse: "0.0.0.0 (blocking response)",
              ApplicableRules: [{...}],
              HasWarning: false
           }

Print formatted output to stdout
```

---

## Service Operation

### Installation Process

**macOS**:
```bash
./distractions-free install
```

Steps:
1. Service framework creates LaunchAgent plist at `~/Library/LaunchAgents/com.github.distractions-free.plist`
2. Sets up auto-start on system boot
3. Requires user to enter password (sudo) for privileged operations

**Windows**:
```bash
./distractions-free install
```

Steps:
1. Service framework creates Windows Service
2. Sets up auto-start on system boot
3. Requires Administrator prompt

### Running as Service

```bash
./distractions-free start
```

The service:
- Runs continuously in background
- Loads config at startup
- Starts scheduler (1-minute ticks)
- Binds to port 53 (requires root/admin)
- Starts web dashboard on port 8040
- Handles system sleep/wake gracefully (1-minute ticker resumption)

### Stopping Service

```bash
./distractions-free stop
```

Triggers program.Stop():
- Restores system DNS to default (networksetup)
- Flushes DNS cache (dscacheutil)
- Kills mDNSResponder to apply changes
- All done automatically!

### Accessing Dashboard

While service is running:
```
http://127.0.0.1:8040/api/config  (JSON API)
```

### Non-Service Mode

```bash
./distractions-free --no-service
```

Runs exactly like service but in foreground:
- No privilege escalation needed
- Uses `./config.json` instead of system paths
- Useful for testing or development

---

## Privileged Operations

### Why Root/Admin is Required

1. **Port 53 Binding** (DNS)
   - Ports < 1024 require root on Unix/macOS
   - On Windows, service requires Administrator
   - Cannot be run as regular user

2. **System DNS Configuration**
   - modifying system-wide DNS settings requires elevated privileges
   - On macOS: `networksetup` needs admin
   - On Windows: PowerShell requires Administrator

3. **System Service Registration**
   - LaunchAgent (macOS) requires admin
   - Windows Service requires Administrator

### Privileged Operations in Code

**In proxy/dns.go**:
```go
func StartDNSServer() {
    // This will fail without root/admin
    server := &dns.Server{Addr: "127.0.0.1:53", Net: "udp"}
    if err := server.ListenAndServe(); err != nil {
        // "listen udp 127.0.0.1:53: permission denied"
        log.Fatalf("Failed to start DNS server: %s", err.Error())
    }
}
```

**In scheduler/scheduler.go (macOS)**:
```go
func evaluateRules() {
    // ... when block starts ...
    if runtime.GOOS == "darwin" {
        // This requires user to be logged in + have access to Chrome/Safari
        exec.Command("osascript", "-e", script).Run()
    }
}
```

**In scheduler/scheduler.go (Stop)**:
```go
func (p *program) Stop(s service.Service) error {
    if runtime.GOOS == "darwin" {
        // These require root/admin to execute
        exec.Command("networksetup", "-setdnsservers", "Wi-Fi", "Empty").Run()
        exec.Command("dscacheutil", "-flushcache").Run()
        exec.Command("killall", "-HUP", "mDNSResponder").Run()
    }
    return nil
}
```

### Operations WITHOUT Privileges

The testing architecture allows comprehensive testing without root:

**Testable Functions**:
- `scheduler.EvaluateRulesAtTime()` - Pure function, no system access
- `proxy.GetDNSResponse()` - No port binding, queries upstream DNS
- Web handlers - Run on :8040 (no special privileges)

**Why This Works**:
- Testable functions accept parameters (time, blocked list) instead of calling system functions
- No mocking needed - real upstream DNS servers respond to test queries
- No port binding - tests use the functions directly
- Thread-safe - tests can run in parallel

---

## Testing Architecture

### Test Strategy

All tests run **without privileges**, **without mocking**, and **without port binding**.

### Unit Test Structure

**scheduler_test.go** (17 tests):
```go
func TestEvaluateRulesAtTime_DomainBlockedDuringSchedule(t *testing.T) {
    cfg := config.Config{
        Rules: []config.Rule{
            {
                Domain: "youtube.com",
                IsActive: true,
                Schedules: map[string][]config.TimeSlot{
                    "Monday": {{Start: "09:00", End: "17:00"}},
                },
            },
        },
    }
    
    // Test specific time: Monday 10:30 (within block)
    testTime := time.Date(2024, 4, 1, 10, 30, 0, 0, time.UTC)
    
    blocked := scheduler.EvaluateRulesAtTime(testTime, cfg)
    
    if !blocked["youtube.com"] {
        t.Error("youtube.com should be blocked at Monday 10:30")
    }
}
```

**proxy_test.go** (16 tests):
```go
func TestGetDNSResponse_AllowedDomainForwarded(t *testing.T) {
    blocked := map[string]bool{}  // Empty - nothing blocked
    
    query := new(dns.Msg)
    query.SetQuestion("google.com.", dns.TypeA)
    
    response, err := proxy.GetDNSResponse(
        query, 
        blocked,
        "8.8.8.8:53",      // Real upstream
        "1.1.1.1:53",      // Real backup
    )
    
    if err != nil {
        t.Fatalf("DNS query failed: %v", err)
    }
    
    if len(response.Answer) == 0 {
        t.Error("Should have DNS response")
    }
    
    // Verify response is from real Google DNS
    // (actual IP address, not 0.0.0.0)
}
```

**web_test.go** (16 tests):
```go
func TestConfigHandler_ReturnsJSON(t *testing.T) {
    req := httptest.NewRequest("GET", "/api/config", nil)
    w := httptest.NewRecorder()
    
    ConfigHandler(w, req)
    
    if w.Header().Get("Content-Type") != "application/json" {
        t.Error("Should return application/json")
    }
    
    if w.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", w.Code)
    }
}
```

**testcli_test.go** (22 tests):
```go
func TestQueryBlocking_ValidTimeFormat(t *testing.T) {
    config.UseLocalConfig = true
    err := testcli.QueryBlocking("2024-04-01 10:30", "google.com")
    if err != nil {
        t.Errorf("Valid time format failed: %v", err)
    }
}
```

### Running Tests

```bash
# All tests
go test ./internal/... -v

# Specific package
go test ./internal/scheduler -v

# Specific test
go test ./internal/scheduler -run TestEvaluateRulesAtTime

# Coverage
go test ./internal/... -cover

# Parallel execution
go test -race ./internal/...
```

### Test Results (71 Total Tests)

```
ok      github.com/vsangava/distractions-free/internal/proxy      0.393s
ok      github.com/vsangava/distractions-free/internal/scheduler   0.364s
ok      github.com/vsangava/distractions-free/internal/testcli     0.938s
ok      github.com/vsangava/distractions-free/internal/web         0.506s
```

---

## Configuration

### Config File Format

Location: `/Library/Application Support/DistractionsFree/config.json` (macOS)

```json
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
        "Monday": [
          {"start": "09:00", "end": "17:00"}
        ],
        "Tuesday": [
          {"start": "09:00", "end": "17:00"}
        ],
        "Wednesday": [
          {"start": "09:00", "end": "17:00"}
        ],
        "Thursday": [
          {"start": "09:00", "end": "17:00"}
        ],
        "Friday": [
          {"start": "09:00", "end": "17:00"}
        ]
      }
    },
    {
      "domain": "reddit.com",
      "is_active": true,
      "schedules": {
        "Monday": [
          {"start": "14:00", "end": "14:30"}
        ]
      }
    },
    {
      "domain": "twitter.com",
      "is_active": false,
      "schedules": {
        "Monday": [
          {"start": "09:00", "end": "17:00"}
        ]
      }
    }
  ]
}
```

### Config Reloading

The service **automatically detects config changes** every minute:

```go
func evaluateRules() {
    cfg := config.GetConfig()  // Always loads latest from disk
    // ... rest of evaluation ...
}
```

So you can:
1. Edit `config.json`
2. Save file
3. Wait up to 1 minute
4. New rules take effect automatically (no service restart needed)

---

## Mode of Operation

### 1. Service Mode (Production)

```bash
# Installation
sudo ./distractions-free install

# Start service
sudo ./distractions-free start

# Verify running
sudo ./distractions-free status
```

**What happens**:
- Service runs continuously as root/admin
- Binds to port 53 for DNS
- Scheduler evaluates every minute
- Web dashboard accessible at 127.0.0.1:8040
- Config at system path (`/Library/Application Support/...`)

---

### 2. No-Service Mode (Local Development)

```bash
./distractions-free --no-service
```

**What happens**:
- Runs in foreground
- Uses `./config.json` (current directory)
- Requires NO privileges
- Perfect for development and testing

---

### 3. CLI Test Mode (Quick Testing)

```bash
./distractions-free --test-query "2024-04-01 10:30" youtube.com
```

**Output**:
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
⚠️ Warning will trigger 3 minutes before block!
============================================================
```

**What happens**:
- No service needed
- No privileges required
- Uses `./config.json`
- Queries real upstream DNS
- Exits immediately

---

### 4. Web UI Test Mode (Interactive Testing)

```bash
./distractions-free --test-web
```

**What happens**:
- Starts web server at 127.0.0.1:8040
- Opens beautiful interactive UI in browser
- Real-time DNS queries
- Displays config and results
- No privileges required

---

## Request/Response Examples

### API: Get Current Config

```bash
curl http://127.0.0.1:8040/api/config
```

Response:
```json
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
        "Monday": [{"start": "09:00", "end": "17:00"}]
      }
    }
  ]
}
```

### API: Test Query (Blocked)

```bash
curl 'http://127.0.0.1:8040/api/test-query?time=2024-04-01%2010:30&domain=youtube.com'
```

Response:
```json
{
  "time": "2024-04-01 10:30",
  "weekday": "Monday",
  "domain": "youtube.com",
  "is_blocked": true,
  "blocking_status": "🚫 BLOCKED",
  "dns_response": "0.0.0.0 (blocking response)",
  "applicable_rules": [
    {
      "domain": "youtube.com",
      "schedules": [
        {
          "weekday": "Monday",
          "start": "09:00",
          "end": "17:00",
          "is_active": true
        }
      ]
    }
  ],
  "has_warning": false
}
```

### API: Test Query (Allowed)

```bash
curl 'http://127.0.0.1:8040/api/test-query?time=2024-04-06%2010:30&domain=google.com'
```

Response:
```json
{
  "time": "2024-04-06 10:30",
  "weekday": "Saturday",
  "domain": "google.com",
  "is_blocked": false,
  "blocking_status": "✓ ALLOWED (forwarded to upstream DNS)",
  "dns_response": "google.com. 287 IN A 142.251.215.110",
  "applicable_rules": null,
  "has_warning": false
}
```

---

## Summary

**Distractions-Free** implements a comprehensive system-level DNS proxy with:

1. **Three-tier architecture**:
   - Scheduler (evaluates rules every minute)
   - DNS Proxy (intercepts port 53)
   - Web UI (dashboard and testing)

2. **Testable design**:
   - Pure functions for core logic
   - No mocking needed
   - Real DNS queries in tests

3. **Multiple operating modes**:
   - Service mode (production)
   - No-service mode (development)
   - CLI test mode (quick verification)
   - Web UI test mode (interactive testing)

4. **Production-ready**:
   - Thread-safe operations
   - Error handling and fallbacks
   - Automatic config reloading
   - Clean shutdown and DNS restoration

package testcli

import (
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/proxy"
	"github.com/vsangava/distractions-free/internal/scheduler"
)

const timeFormat = "2006-01-02 15:04"

// QueryResult holds the structured result of a blocking query.
type QueryResult struct {
	Time            string     `json:"time"`
	Weekday         string     `json:"weekday"`
	Domain          string     `json:"domain"`
	IsBlocked       bool       `json:"is_blocked"`
	BlockingStatus  string     `json:"blocking_status"`
	DNSResponse     string     `json:"dns_response"`
	ApplicableRules []RuleInfo `json:"applicable_rules"`
	HasWarning      bool       `json:"has_warning"`
	WarningMessage  string     `json:"warning_message,omitempty"`
	Error           string     `json:"error,omitempty"`
}

// RuleInfo holds information about an applicable blocking rule.
type RuleInfo struct {
	Domain    string         `json:"domain"`
	Schedules []ScheduleInfo `json:"schedules"`
}

// ScheduleInfo holds schedule information.
type ScheduleInfo struct {
	Weekday  string `json:"weekday"`
	Start    string `json:"start"`
	End      string `json:"end"`
	IsActive bool   `json:"is_active"`
}

// GetQueryResult returns structured test query result without printing.
func GetQueryResult(timeStr, domain string) QueryResult {
	// Load config
	if err := config.LoadConfig(); err != nil {
		return QueryResult{
			Time:   timeStr,
			Domain: domain,
			Error:  fmt.Sprintf("Failed to load config: %v", err),
		}
	}

	cfg := config.GetConfig()
	return GetQueryResultWithConfig(timeStr, domain, cfg)
}

// GetQueryResultWithConfig returns structured test query result using provided config.
func GetQueryResultWithConfig(timeStr, domain string, cfg config.Config) QueryResult {
	result := QueryResult{
		Time:   timeStr,
		Domain: domain,
	}

	// Parse the time
	testTime, err := time.Parse(timeFormat, timeStr)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid time format. Use: %s (example: 2024-04-01 10:30)", timeFormat)
		return result
	}

	result.Weekday = testTime.Weekday().String()

	// Normalize domain
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		result.Error = "Domain cannot be empty"
		return result
	}
	result.Domain = domain

	// Evaluate blocking rules at this time
	blockedDomains := scheduler.EvaluateRulesAtTime(testTime, cfg)
	result.IsBlocked = proxy.IsDomainBlocked(domain, blockedDomains)

	// Create a DNS query
	dnsQuery := new(dns.Msg)
	dnsQuery.SetQuestion(domain+".", dns.TypeA)

	// Get the DNS response
	response, err := proxy.GetDNSResponse(dnsQuery, blockedDomains, cfg.Settings.PrimaryDNS, cfg.Settings.BackupDNS)
	if err != nil {
		result.Error = fmt.Sprintf("DNS query failed: %v", err)
		return result
	}

	// Set blocking status
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
		} else {
			result.DNSResponse = "No DNS answer (domain may not exist or DNS error)"
		}
	}

	// Find applicable rules — includes parent-domain rules (e.g. youtube.com rule applies to m.youtube.com)
	now := time.Date(0, 1, 1, testTime.Hour(), testTime.Minute(), 0, 0, time.UTC)
	for _, rule := range cfg.Rules {
		if !rule.IsActive {
			continue
		}
		if !proxy.IsDomainBlocked(domain, map[string]bool{rule.Domain: true}) {
			continue
		}

		ruleInfo := RuleInfo{Domain: rule.Domain}

		if slots, exists := rule.Schedules[testTime.Weekday().String()]; exists {
			for _, slot := range slots {
				slotStart, errS := time.Parse("15:04", slot.Start)
				slotEnd, errE := time.Parse("15:04", slot.End)
				var isActive bool
				if errS == nil && errE == nil {
					isActive = (now.Equal(slotStart) || now.After(slotStart)) && now.Before(slotEnd)
				}
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

	// Check for warnings — a subdomain triggers a warning if its parent domain has one
	warnings := scheduler.CheckWarningDomainsAtTime(testTime, cfg)
	for _, w := range warnings {
		if proxy.IsDomainBlocked(domain, map[string]bool{w: true}) {
			result.HasWarning = true
			result.WarningMessage = "⚠️ Warning will trigger 3 minutes before block!"
			break
		}
	}

	return result
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// QueryBlocking tests whether a domain would be blocked at a specific time,
// and returns the DNS response (either 0.0.0.0 or upstream result).
func QueryBlocking(timeStr, domain string) error {
	// Parse the time
	testTime, err := time.Parse(timeFormat, timeStr)
	if err != nil {
		return fmt.Errorf("invalid time format. Use: %s (example: 2024-04-01 10:30)", timeFormat)
	}

	// Normalize domain (remove trailing dot if present)
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// Load config
	if err := config.LoadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	cfg := config.GetConfig()

	// Evaluate blocking rules at this time
	blockedDomains := scheduler.EvaluateRulesAtTime(testTime, cfg)

	// Create a DNS query
	dnsQuery := new(dns.Msg)
	dnsQuery.SetQuestion(domain+".", dns.TypeA)

	// Get the DNS response
	response, err := proxy.GetDNSResponse(dnsQuery, blockedDomains, cfg.Settings.PrimaryDNS, cfg.Settings.BackupDNS)
	if err != nil {
		return fmt.Errorf("DNS query failed: %v", err)
	}

	// Format and print results
	isBlocked := proxy.IsDomainBlocked(domain, blockedDomains)

	separator := strings.Repeat("=", 60)
	dashLine := strings.Repeat("-", 60)

	fmt.Println(separator)
	fmt.Printf("Test Query Result\n")
	fmt.Println(separator)
	fmt.Printf("Time:          %s (%s)\n", testTime.Format(timeFormat), testTime.Weekday())
	fmt.Printf("Domain:        %s\n", domain)
	fmt.Println(dashLine)

	if isBlocked {
		fmt.Printf("Status:        🚫 BLOCKED\n")
		if len(response.Answer) > 0 {
			if a, ok := response.Answer[0].(*dns.A); ok {
				fmt.Printf("Response:      %s (blocking response)\n", a.A.String())
			}
		}
	} else {
		fmt.Printf("Status:        ✓ ALLOWED (forwarded to upstream DNS)\n")
		if len(response.Answer) > 0 {
			fmt.Printf("Response:      %v\n", response.Answer[0])
		} else {
			fmt.Printf("Response:      No DNS answer (domain may not exist or DNS error)\n")
		}
	}

	// Show blocking rules that apply
	fmt.Println(dashLine)
	fmt.Println("Applicable Rules:")

	now := time.Date(0, 1, 1, testTime.Hour(), testTime.Minute(), 0, 0, time.UTC)
	foundRules := false
	for _, rule := range cfg.Rules {
		if !rule.IsActive {
			continue
		}
		if !proxy.IsDomainBlocked(domain, map[string]bool{rule.Domain: true}) {
			continue
		}

		foundRules = true
		fmt.Printf("  Domain: %s\n", rule.Domain)

		if slots, exists := rule.Schedules[testTime.Weekday().String()]; exists {
			for _, slot := range slots {
				slotStart, errS := time.Parse("15:04", slot.Start)
				slotEnd, errE := time.Parse("15:04", slot.End)
				active := errS == nil && errE == nil &&
					(now.Equal(slotStart) || now.After(slotStart)) && now.Before(slotEnd)
				if active {
					fmt.Printf("    ✓ Blocked on %s from %s to %s (ACTIVE)\n", testTime.Weekday(), slot.Start, slot.End)
				} else {
					fmt.Printf("    ○ Blocked on %s from %s to %s (not active now)\n", testTime.Weekday(), slot.Start, slot.End)
				}
			}
		} else {
			fmt.Printf("    ○ No schedule for %s\n", testTime.Weekday())
		}
	}

	if !foundRules {
		fmt.Println("  (No active rules for this domain)")
	}

	// Show warning info
	fmt.Println(dashLine)
	warnings := scheduler.CheckWarningDomainsAtTime(testTime, cfg)
	for _, w := range warnings {
		if proxy.IsDomainBlocked(domain, map[string]bool{w: true}) {
			fmt.Printf("⚠️  Warning will trigger 3 minutes before block!\n")
			break
		}
	}

	fmt.Println(separator)

	return nil
}


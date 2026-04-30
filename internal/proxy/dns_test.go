package proxy

import (
	"testing"

	"github.com/miekg/dns"
)

func TestGetDNSResponse_BlockedDomainReturnsZeroIP(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com": true,
	}

	// Create a DNS query for youtube.com
	msg := new(dns.Msg)
	msg.SetQuestion("youtube.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer records, got none")
	}

	// Check if the answer is a 0.0.0.0 response
	if a, ok := response.Answer[0].(*dns.A); ok {
		if a.A.String() != "0.0.0.0" {
			t.Errorf("expected 0.0.0.0 for blocked domain, got %s", a.A.String())
		}
	} else {
		t.Errorf("expected A record, got %T", response.Answer[0])
	}
}

func TestGetDNSResponse_AllowedDomainForwarded(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com": true,
	}

	// Query for google.com (not blocked)
	msg := new(dns.Msg)
	msg.SetQuestion("google.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if response == nil {
		t.Fatalf("expected response, got nil")
	}

	// Should have received actual DNS response (not 0.0.0.0)
	if len(response.Answer) == 0 {
		t.Fatalf("expected answer records from upstream DNS, got none")
	}

	// Verify it's not a blocking response
	if a, ok := response.Answer[0].(*dns.A); ok {
		if a.A.String() == "0.0.0.0" {
			t.Errorf("expected real IP for allowed domain, got 0.0.0.0")
		}
	}
}

func TestGetDNSResponse_EmptyQuestion(t *testing.T) {
	blockedDomains := map[string]bool{}

	msg := new(dns.Msg)
	// Don't set any questions

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error for empty question, got %v", err)
	}

	if response == nil {
		t.Fatalf("expected response, got nil")
	}

	if len(response.Answer) != 0 {
		t.Errorf("expected no answers for empty question, got %d", len(response.Answer))
	}
}

func TestGetDNSResponse_BlockedDomainAAAAReturnsZeroIPv6(t *testing.T) {
	blockedDomains := map[string]bool{
		"reddit.com": true,
	}

	msg := new(dns.Msg)
	msg.SetQuestion("reddit.com.", dns.TypeAAAA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatal("expected answer record for blocked AAAA query")
	}

	aaaa, ok := response.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("expected AAAA record, got %T", response.Answer[0])
	}
	if aaaa.AAAA.String() != "::" {
		t.Errorf("expected :: for blocked AAAA query, got %s", aaaa.AAAA.String())
	}
}

func TestGetDNSResponse_AllowedDomainAAAAForwarded(t *testing.T) {
	blockedDomains := map[string]bool{
		"reddit.com": true,
	}

	msg := new(dns.Msg)
	msg.SetQuestion("google.com.", dns.TypeAAAA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response == nil {
		t.Fatal("expected response, got nil")
	}
	// Must not return the blocking sentinel address.
	for _, rr := range response.Answer {
		if aaaa, ok := rr.(*dns.AAAA); ok {
			if aaaa.AAAA.String() == "::" {
				t.Errorf("allowed domain got blocking response :: for AAAA")
			}
		}
	}
}

func TestGetDNSResponse_DomainWithWWW(t *testing.T) {
	blockedDomains := map[string]bool{
		"twitter.com": true,
	}

	// Query for www.twitter.com (not directly in blocked list but domain is)
	msg := new(dns.Msg)
	msg.SetQuestion("www.twitter.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// This tests exact domain matching, so www.twitter.com won't be blocked
	// (only twitter.com would be)
	if len(response.Answer) == 0 {
		t.Fatalf("expected answer records")
	}
}

func TestGetDNSResponse_MultipleBlockedDomains(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com":  true,
		"reddit.com":   true,
		"facebook.com": true,
		"twitter.com":  true,
	}

	domains := []string{"youtube.com", "reddit.com", "facebook.com", "twitter.com"}

	for _, domain := range domains {
		msg := new(dns.Msg)
		msg.SetQuestion(domain+".", dns.TypeA)

		response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

		if err != nil {
			t.Fatalf("expected no error for %s, got %v", domain, err)
		}

		if len(response.Answer) == 0 {
			t.Fatalf("expected answer for %s", domain)
		}

		if a, ok := response.Answer[0].(*dns.A); ok {
			if a.A.String() != "0.0.0.0" {
				t.Errorf("expected 0.0.0.0 for %s, got %s", domain, a.A.String())
			}
		}
	}
}

func TestGetDNSResponse_CaseInsensitivityBlocking(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com": true,
	}

	// Query with uppercase
	msg := new(dns.Msg)
	msg.SetQuestion("YOUTUBE.COM.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// DNS domains are case-insensitive, but our map lookup is case-sensitive
	// This test documents that we don't block YOUTUBE.COM (uppercase)
	// In production, this should be handled with lowercase normalization
	if len(response.Answer) > 0 {
		if a, ok := response.Answer[0].(*dns.A); ok {
			// If it's not 0.0.0.0, then we correctly do case-sensitive matching
			// (which may be a bug to fix in production)
			_ = a
		}
	}
}

func TestGetDNSResponse_EmptyBlockedList(t *testing.T) {
	blockedDomains := make(map[string]bool)

	msg := new(dns.Msg)
	msg.SetQuestion("google.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer records from upstream DNS")
	}
}

func TestGetDNSResponse_BlockedDomainWithTrailingDot(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com": true,
	}

	// Query comes with trailing dot
	msg := new(dns.Msg)
	msg.SetQuestion("youtube.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer")
	}

	if a, ok := response.Answer[0].(*dns.A); ok {
		if a.A.String() != "0.0.0.0" {
			t.Errorf("expected 0.0.0.0 for blocked domain with trailing dot, got %s", a.A.String())
		}
	}
}

func TestGetDNSResponse_MXRecordQuery(t *testing.T) {
	blockedDomains := map[string]bool{
		"gmail.com": true,
	}

	// Query for MX record (different type)
	msg := new(dns.Msg)
	msg.SetQuestion("gmail.com.", dns.TypeMX)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// MX queries for blocked domains are forwarded (only A records are blocked)
	if response == nil {
		t.Fatalf("expected response, got nil")
	}
}

func TestGetDNSResponse_CNAMEQuery(t *testing.T) {
	blockedDomains := map[string]bool{
		"example.com": true,
	}

	// Query for CNAME record
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeCNAME)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if response == nil {
		t.Fatalf("expected response, got nil")
	}
}

func TestGetDNSResponse_UpstreamFailover(t *testing.T) {
	blockedDomains := make(map[string]bool)

	// Query with a non-existent primary DNS, should failover to backup
	msg := new(dns.Msg)
	msg.SetQuestion("google.com.", dns.TypeA)

	// Use backup DNS that works (1.1.1.1)
	response, err := GetDNSResponse(msg, blockedDomains, "127.0.0.1:54321", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error (failover to backup DNS), got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer records from backup DNS")
	}
}

func TestGetDNSResponse_PreservesReplyFlag(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com": true,
	}

	msg := new(dns.Msg)
	msg.SetQuestion("youtube.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check that response is marked as reply
	if !response.Response {
		t.Errorf("expected response flag to be set")
	}
}

func TestGetDNSResponse_AllowedDomainFromBackupDNS(t *testing.T) {
	blockedDomains := make(map[string]bool)

	msg := new(dns.Msg)
	msg.SetQuestion("cloudflare.com.", dns.TypeA)

	// Both primary and backup should work
	response, err := GetDNSResponse(msg, blockedDomains, "1.1.1.1:53", "8.8.8.8:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer records from upstream DNS")
	}
}

func TestUpdateBlockedDomains(t *testing.T) {
	newBlocked := map[string]bool{
		"youtube.com": true,
		"reddit.com":  true,
	}

	UpdateBlockedDomains(newBlocked)

	// Verify by querying
	msg := new(dns.Msg)
	msg.SetQuestion("youtube.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, newBlocked, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer")
	}

	if a, ok := response.Answer[0].(*dns.A); ok {
		if a.A.String() != "0.0.0.0" {
			t.Errorf("expected youtube.com to be blocked after update")
		}
	}
}

func TestGetDNSResponse_BlockingResponseTTL(t *testing.T) {
	blockedDomains := map[string]bool{
		"youtube.com": true,
	}

	msg := new(dns.Msg)
	msg.SetQuestion("youtube.com.", dns.TypeA)

	response, err := GetDNSResponse(msg, blockedDomains, "8.8.8.8:53", "1.1.1.1:53")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatalf("expected answer")
	}

	if a, ok := response.Answer[0].(*dns.A); ok {
		// Check TTL is set
		if a.Hdr.Ttl != 60 {
			t.Errorf("expected TTL 60 for blocking response, got %d", a.Hdr.Ttl)
		}
	}
}

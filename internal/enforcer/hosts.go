package enforcer

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/vsangava/sentinel/internal/config"
)

const (
	blockBegin  = "# sentinel:begin"
	blockEnd    = "# sentinel:end"
	blockingIP  = "0.0.0.0"
	blockingIP6 = "::1"
)

// subdomainPrefixes are prepended to each blocked domain because /etc/hosts
// has no wildcard support. Keep this list to the most common prefixes to avoid
// bloating the hosts file.
var subdomainPrefixes = []string{"", "www.", "m.", "mobile.", "app."}

// GenerateHostsEntries returns the /etc/hosts lines that would be written for
// the given domains — one line per (domain × prefix) combination. No file is
// read or written; this is a pure preview function used for testing and the
// web UI hosts-preview endpoint.
func GenerateHostsEntries(domains []string) []string {
	var entries []string
	for _, domain := range domains {
		for _, prefix := range subdomainPrefixes {
			entries = append(entries, blockingIP+" "+prefix+domain)
			entries = append(entries, blockingIP6+" "+prefix+domain)
		}
	}
	return entries
}

// HostsEnforcer blocks domains by writing entries into /etc/hosts inside a
// clearly-marked managed section, leaving all other entries untouched.
type HostsEnforcer struct {
	hostsPath string
	cfg       config.Config
}

func NewHostsEnforcer(cfg config.Config) *HostsEnforcer {
	return &HostsEnforcer{hostsPath: defaultHostsPath(), cfg: cfg}
}

func defaultHostsPath() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

func (e *HostsEnforcer) Setup() error {
	return nil
}

func (e *HostsEnforcer) Teardown() error {
	return e.DeactivateAll()
}

// Activate adds the domains (plus common subdomains) to the managed hosts section.
// The operation is idempotent: already-present entries are skipped.
func (e *HostsEnforcer) Activate(domains []string) error {
	lines, err := readHostsFile(e.hostsPath)
	if err != nil {
		return fmt.Errorf("hosts activate: %w", err)
	}

	existing := parseExistingBlocks(lines)
	var newEntries []string
	for _, domain := range domains {
		for _, prefix := range subdomainPrefixes {
			for _, ip := range []string{blockingIP, blockingIP6} {
				entry := ip + " " + prefix + domain
				if !existing[entry] {
					newEntries = append(newEntries, entry)
				}
			}
		}
	}

	if len(newEntries) == 0 {
		return nil
	}

	updated := injectEntries(lines, newEntries)
	if err := writeHostsFile(e.hostsPath, updated); err != nil {
		return fmt.Errorf("hosts activate: %w", err)
	}

	flushDNSCache()
	log.Printf("hosts: activated %v", domains)
	return nil
}

// Deactivate removes the given domains from the managed hosts section.
func (e *HostsEnforcer) Deactivate(domains []string) error {
	lines, err := readHostsFile(e.hostsPath)
	if err != nil {
		return fmt.Errorf("hosts deactivate: %w", err)
	}

	toRemove := make(map[string]bool)
	for _, domain := range domains {
		for _, prefix := range subdomainPrefixes {
			for _, ip := range []string{blockingIP, blockingIP6} {
				toRemove[ip+" "+prefix+domain] = true
			}
		}
	}

	updated := removeEntries(lines, toRemove)
	if err := writeHostsFile(e.hostsPath, updated); err != nil {
		return fmt.Errorf("hosts deactivate: %w", err)
	}

	flushDNSCache()
	log.Printf("hosts: deactivated %v", domains)
	return nil
}

// DeactivateAll removes the entire managed block section from the hosts file,
// restoring it to its pre-installation state.
func (e *HostsEnforcer) DeactivateAll() error {
	lines, err := readHostsFile(e.hostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("hosts deactivate-all: %w", err)
	}

	updated := removeAllManagedEntries(lines)
	if err := writeHostsFile(e.hostsPath, updated); err != nil {
		return fmt.Errorf("hosts deactivate-all: %w", err)
	}

	flushDNSCache()
	log.Println("hosts: removed all managed entries")
	return nil
}

func readHostsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// writeHostsFile writes atomically via a temp file + rename to prevent
// partial writes from corrupting the hosts file.
func writeHostsFile(path string, lines []string) error {
	tmp := path + ".df.tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func parseExistingBlocks(lines []string) map[string]bool {
	existing := make(map[string]bool)
	inBlock := false
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case blockBegin:
			inBlock = true
		case blockEnd:
			inBlock = false
		default:
			if inBlock {
				existing[strings.TrimSpace(line)] = true
			}
		}
	}
	return existing
}

// injectEntries inserts new entries before the closing marker of the managed block.
// If no managed block exists yet, one is appended at the end of the file.
func injectEntries(lines []string, newEntries []string) []string {
	beginIdx, endIdx := -1, -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case blockBegin:
			beginIdx = i
		case blockEnd:
			endIdx = i
		}
	}

	if beginIdx >= 0 && endIdx > beginIdx {
		var result []string
		result = append(result, lines[:endIdx]...)
		result = append(result, newEntries...)
		result = append(result, lines[endIdx:]...)
		return result
	}

	// No existing block — append a new one.
	var result []string
	result = append(result, lines...)
	result = append(result, "", blockBegin)
	result = append(result, newEntries...)
	result = append(result, blockEnd)
	return result
}

func removeEntries(lines []string, toRemove map[string]bool) []string {
	inBlock := false
	var result []string
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case blockBegin:
			inBlock = true
			result = append(result, line)
		case blockEnd:
			inBlock = false
			result = append(result, line)
		default:
			if inBlock && toRemove[strings.TrimSpace(line)] {
				continue
			}
			result = append(result, line)
		}
	}
	return result
}

// removeAllManagedEntries strips the entire managed block section including its
// markers and the blank line that preceded it.
func removeAllManagedEntries(lines []string) []string {
	inBlock := false
	var result []string
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case blockBegin:
			inBlock = true
		case blockEnd:
			inBlock = false
		default:
			if !inBlock {
				result = append(result, line)
			}
		}
	}
	// Remove the blank separator line that injectEntries prepends to the block.
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}
	return result
}

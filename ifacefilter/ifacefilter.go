// Package ifacefilter matches network interface names against the wildcard
// patterns of the exclusion rules (REQUIREMENTS 3.1.3). The agent and the
// panel both use it so that the two sides never diverge.
//
// Only "*" is a metacharacter: it matches any sequence of characters,
// including none. Matching is case-sensitive and covers the whole name, so
// "eth*" matches "eth0" but not "veth0".
package ifacefilter

import (
	"errors"
	"fmt"
	"strings"
)

// DefaultExclude is the exclusion list both sides fall back to: loopback,
// container, virtual machine, tunnel and VPN interfaces
// (REQUIREMENTS 3.1.3).
var DefaultExclude = []string{
	"lo",
	"docker*", "veth*", "br-*", "cni*", "flannel*", "cali*", "kube-*",
	"virbr*", "vnet*",
	"tun*", "tap*", "wg*", "tailscale*", "zt*",
	"dummy*",
}

// Match reports whether name matches any of the patterns.
func Match(patterns []string, name string) bool {
	for _, p := range patterns {
		if matchOne(p, name) {
			return true
		}
	}
	return false
}

func matchOne(pattern, name string) bool {
	prefix, rest, found := strings.Cut(pattern, "*")
	if !found {
		return pattern == name
	}
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	name = name[len(prefix):]
	// Every remaining literal but the last must appear in order; taking the
	// leftmost occurrence of each is safe because what follows may match any
	// suffix of the rest.
	for {
		lit, more, found := strings.Cut(rest, "*")
		if !found {
			return strings.HasSuffix(name, lit)
		}
		i := strings.Index(name, lit)
		if i < 0 {
			return false
		}
		name = name[i+len(lit):]
		rest = more
	}
}

// ParsePatterns splits a stored comma-separated list. Values reaching it
// have already been validated where they were written, so it is lenient:
// it trims spaces and drops empty entries.
func ParsePatterns(list string) []string {
	var out []string
	for p := range strings.SplitSeq(list, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Limits on a stored exclusion list (ValidatePatterns).
const (
	// MaxPatternLen is the longest a single pattern may be.
	MaxPatternLen = 32
	// MaxPatterns is the most patterns one list may hold.
	MaxPatterns = 64
)

// ValidatePatterns parses a comma-separated exclusion list the way
// ParsePatterns does, but rejects what must never be stored: an empty
// entry, an over-long pattern, one that is not printable ASCII without
// spaces, and a list with too many entries. Both sides validate where the
// list is written -- the agent in its configuration file, the panel in its
// settings -- so that everything read back is already sound.
//
// An empty list is valid and excludes nothing.
func ValidatePatterns(list string) ([]string, error) {
	if strings.TrimSpace(list) == "" {
		return nil, nil
	}
	parts := strings.Split(list, ",")
	if len(parts) > MaxPatterns {
		return nil, fmt.Errorf("ifacefilter: %d patterns, at most %d are allowed", len(parts), MaxPatterns)
	}
	patterns := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
			return nil, errors.New("ifacefilter: the list contains an empty pattern")
		case len(p) > MaxPatternLen:
			return nil, fmt.Errorf("ifacefilter: pattern %q is longer than %d characters", p, MaxPatternLen)
		}
		for _, c := range []byte(p) {
			if c <= ' ' || c > '~' {
				return nil, fmt.Errorf("ifacefilter: pattern %q must be printable ASCII without spaces", p)
			}
		}
		patterns = append(patterns, p)
	}
	return patterns, nil
}

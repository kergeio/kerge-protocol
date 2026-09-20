// Package ifacefilter matches network interface names against the wildcard
// patterns of the exclusion rules (REQUIREMENTS 3.1.3). The agent and the
// panel both use it so that the two sides never diverge.
//
// Only "*" is a metacharacter: it matches any sequence of characters,
// including none. Matching is case-sensitive and covers the whole name, so
// "eth*" matches "eth0" but not "veth0".
package ifacefilter

import "strings"

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

package ifacefilter

import (
	"slices"
	"strings"
	"testing"
)

func TestMatchOne(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"lo", "lo", true},
		{"lo", "lo0", false},
		{"lo", "l", false},
		{"lo", "LO", false},
		{"docker*", "docker0", true},
		{"docker*", "docker", true},
		{"docker*", "mydocker0", false},
		{"br-*", "br-1a2b", true},
		{"br-*", "br", false},
		{"kube-*", "kube-ipvs0", true},
		{"*", "anything", true},
		{"*", "", true},
		{"*eth*", "veth0", true},
		{"*eth*", "eth0", true},
		{"*eth*", "enp3s0", false},
		{"e*h*0", "eth0", true},
		{"e*h*0", "eth1", false},
		{"a*a", "aa", true},
		{"a*a", "a", false},
		{"a*a", "aba", true},
		{"**", "x", true},
		{"", "", true},
		{"", "x", false},
	}
	for _, c := range cases {
		if got := matchOne(c.pattern, c.name); got != c.want {
			t.Errorf("matchOne(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestMatch(t *testing.T) {
	patterns := []string{"lo", "docker*", "veth*", "br-*"}
	for _, name := range []string{"lo", "docker0", "veth1a2b", "br-x"} {
		if !Match(patterns, name) {
			t.Errorf("Match(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"eth0", "enp3s0", "wlan0", "lo0", "bond0"} {
		if Match(patterns, name) {
			t.Errorf("Match(%q) = true, want false", name)
		}
	}
	if Match(nil, "eth0") {
		t.Error("Match with no patterns = true, want false")
	}
}

func TestParsePatterns(t *testing.T) {
	got := ParsePatterns(" lo , docker* ,, veth* ")
	want := []string{"lo", "docker*", "veth*"}
	if !slices.Equal(got, want) {
		t.Errorf("ParsePatterns = %q, want %q", got, want)
	}
	if got := ParsePatterns(""); got != nil {
		t.Errorf("ParsePatterns(\"\") = %q, want nil", got)
	}
}

func TestValidatePatterns(t *testing.T) {
	got, err := ValidatePatterns(" lo , docker* ")
	if err != nil {
		t.Fatalf("ValidatePatterns: %v", err)
	}
	if want := []string{"lo", "docker*"}; !slices.Equal(got, want) {
		t.Errorf("ValidatePatterns = %q, want %q", got, want)
	}
	for _, blank := range []string{"", "   "} {
		got, err := ValidatePatterns(blank)
		if err != nil || got != nil {
			t.Errorf("ValidatePatterns(%q) = %q, %v; want nil, nil", blank, got, err)
		}
	}

	long := strings.Repeat("a", MaxPatternLen+1)
	many := strings.Repeat("lo,", MaxPatterns) + "lo"
	for _, list := range []string{
		"lo,,docker*",  // empty entry
		"lo, ,docker*", // entry of spaces only
		",lo",          // leading separator
		"lo,",          // trailing separator
		long,           // pattern too long
		many,           // too many patterns
		"eth 0",        // space inside a pattern
		"eth\t0",       // control character
		"eth\x000",     // NUL
		"ethé",         // not ASCII
	} {
		if got, err := ValidatePatterns(list); err == nil {
			t.Errorf("ValidatePatterns(%q) = %q, nil; want an error", list, got)
		}
	}
	if _, err := ValidatePatterns(strings.Repeat("lo,", MaxPatterns-1) + "lo"); err != nil {
		t.Errorf("ValidatePatterns with %d patterns: %v", MaxPatterns, err)
	}
	if _, err := ValidatePatterns(strings.Repeat("a", MaxPatternLen)); err != nil {
		t.Errorf("ValidatePatterns with a pattern of %d characters: %v", MaxPatternLen, err)
	}
}

// TestDefaultExcludeIsValid keeps the built-in list within the limits the
// panel and the agent enforce on what an operator may type.
func TestDefaultExcludeIsValid(t *testing.T) {
	if _, err := ValidatePatterns(strings.Join(DefaultExclude, ",")); err != nil {
		t.Errorf("DefaultExclude: %v", err)
	}
}

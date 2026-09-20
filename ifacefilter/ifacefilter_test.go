package ifacefilter

import "testing"

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

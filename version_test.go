package protocol

import "testing"

func TestSelectVersion(t *testing.T) {
	cases := []struct {
		name    string
		offered []string
		want    string
	}{
		{"single value", []string{Version}, Version},
		{"comma separated", []string{"kerge.v9, " + Version}, Version},
		{"several header lines", []string{"kerge.v9", Version}, Version},
		{"nothing offered", nil, ""},
		{"empty value", []string{""}, ""},
		{"unknown version", []string{"kerge.v9"}, ""},
		{"other protocol", []string{"graphql-ws"}, ""},
		{"wrong case", []string{"KERGE.V1"}, ""},
		{"prefix only", []string{"kerge.v1x"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := SelectVersion(c.offered)
			if got != c.want || ok != (c.want != "") {
				t.Errorf("SelectVersion(%q) = %q, %v; want %q", c.offered, got, ok, c.want)
			}
		})
	}
}

func TestSupports(t *testing.T) {
	if !Supports(Version) {
		t.Errorf("Supports(%q) = false", Version)
	}
	// A panel that agrees to nothing echoes an empty subprotocol, which
	// the agent must not read as agreement.
	if Supports("") {
		t.Error(`Supports("") = true`)
	}
	if Supports("kerge.v9") {
		t.Error(`Supports("kerge.v9") = true`)
	}
}

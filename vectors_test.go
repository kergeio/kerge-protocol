package protocol

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// vectorFile is the conformance suite another implementation can run
// against its own decoder; see PROTOCOL.md.
const vectorFile = "testdata/vectors.json"

type vectors struct {
	Version string   `json:"version"`
	About   string   `json:"about"`
	Cases   []vector `json:"cases"`
}

type vector struct {
	Name      string          `json:"name"`
	Direction string          `json:"direction"`
	Input     string          `json:"input"`
	Accept    bool            `json:"accept"`
	Canonical json.RawMessage `json:"canonical"`
	Note      string          `json:"note"`
}

// TestVectors keeps this implementation and the published vectors in step:
// a rule that changes here has to change there too.
func TestVectors(t *testing.T) {
	data, err := os.ReadFile(vectorFile)
	if err != nil {
		t.Fatal(err)
	}
	var suite vectors
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatal(err)
	}
	if suite.Version != Version {
		t.Fatalf("vector version = %q, want %q", suite.Version, Version)
	}
	if len(suite.Cases) == 0 {
		t.Fatal("no cases")
	}

	seen := make(map[string]bool, len(suite.Cases))
	directions := make(map[string]int)
	for _, c := range suite.Cases {
		if seen[c.Name] {
			t.Errorf("duplicate case name %q", c.Name)
		}
		seen[c.Name] = true
		if c.Note == "" {
			t.Errorf("case %q has no note", c.Name)
		}
		directions[c.Direction]++
		t.Run(c.Name, func(t *testing.T) { runVector(t, c) })
	}
	for _, d := range []string{"agent_to_panel", "panel_to_agent"} {
		if directions[d] == 0 {
			t.Errorf("no %s cases", d)
		}
	}
	if n := len(directions); n != 2 {
		t.Errorf("directions = %v, want the two of them", directions)
	}
}

func runVector(t *testing.T, c vector) {
	t.Helper()
	var msg any
	var err error
	switch c.Direction {
	case "agent_to_panel":
		msg, err = DecodeAgentMessage([]byte(c.Input))
	case "panel_to_agent":
		msg, err = DecodePanelMessage([]byte(c.Input))
	default:
		t.Fatalf("unknown direction %q", c.Direction)
	}
	if !c.Accept {
		if err == nil {
			t.Fatalf("the message was accepted: %s", c.Note)
		}
		if c.Canonical != nil {
			t.Error("a rejected case has no canonical form")
		}
		return
	}
	if err != nil {
		t.Fatalf("the message was rejected (%v): %s", err, c.Note)
	}
	if c.Canonical == nil {
		return
	}
	encoded, err := Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(c.Canonical, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("re-encoded to %s, want %s", encoded, c.Canonical)
	}
}

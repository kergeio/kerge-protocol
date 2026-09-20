package protocol

import (
	"strconv"
	"strings"
	"testing"
)

const validMetrics = `{"type":"metrics","ts":1726560000,"mono_ms":86400123,
	"cpu_percent":12.5,"mem_total":2048000000,"mem_used":812000000,
	"swap_total":1073741824,"swap_used":0,"load1":0.42,"load5":0.35,"load15":0.3,
	"disk_total":40000000000,"disk_used":12000000000,"disk_free":26000000000,
	"net":{"eth0":{"rx":123456789,"tx":987654321}},"uptime":864000}`

const validHostInfo = `{"type":"host_info","hostname":"web-1","os":"linux",
	"platform":"debian","platform_version":"12","kernel":"6.1.0","arch":"amd64",
	"cpu_model":"Xeon","cpu_cores":2,"agent_version":"0.1.0","interval_ms":5000}`

func TestDecodeAgentMessage(t *testing.T) {
	m, err := DecodeAgentMessage([]byte(validMetrics))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := m.(*Metrics)
	if !ok {
		t.Fatalf("got %T, want *Metrics", m)
	}
	if got.TS != 1726560000 || got.MonoMS != 86400123 {
		t.Errorf("ts/mono_ms = %d/%d", got.TS, got.MonoMS)
	}
	if got.CPUPercent == nil || *got.CPUPercent != 12.5 {
		t.Errorf("cpu_percent = %v", got.CPUPercent)
	}
	if n := got.Net["eth0"]; n.RX != 123456789 || n.TX != 987654321 {
		t.Errorf("net[eth0] = %+v", n)
	}

	h, err := DecodeAgentMessage([]byte(validHostInfo))
	if err != nil {
		t.Fatal(err)
	}
	info, ok := h.(*HostInfo)
	if !ok {
		t.Fatalf("got %T, want *HostInfo", h)
	}
	if info.Hostname != "web-1" || info.CPUCores != 2 || info.IntervalMS != 5000 {
		t.Errorf("host_info = %+v", info)
	}
}

// An omitted field must stay nil, not become zero: the panel records "no
// data this cycle" rather than 0.
func TestOmittedFieldsStayAbsent(t *testing.T) {
	m, err := DecodeAgentMessage([]byte(`{"type":"metrics","ts":1,"mono_ms":2}`))
	if err != nil {
		t.Fatal(err)
	}
	got := m.(*Metrics)
	if got.CPUPercent != nil || got.MemTotal != nil || got.Load1 != nil ||
		got.DiskTotal != nil || got.Uptime != nil || got.Net != nil {
		t.Errorf("absent fields decoded as present: %+v", got)
	}
}

func TestDecodeAgentMessageRejects(t *testing.T) {
	cases := map[string]string{
		"unknown type":      `{"type":"command","run":"rm -rf /"}`,
		"empty type":        `{"ts":1,"mono_ms":2}`,
		"panel type":        `{"type":"registered","agent_id":"a","secret":"b"}`,
		"unknown field":     `{"type":"metrics","ts":1,"mono_ms":2,"exec":"sh"}`,
		"wrong field type":  `{"type":"metrics","ts":"soon","mono_ms":2}`,
		"negative unsigned": `{"type":"metrics","ts":1,"mono_ms":2,"mem_total":-1,"mem_used":0}`,
		"negative ts":       `{"type":"metrics","ts":-1,"mono_ms":2}`,
		"negative mono":     `{"type":"metrics","ts":1,"mono_ms":-2}`,
		"trailing data":     `{"type":"metrics","ts":1,"mono_ms":2} {}`,
		"not an object":     `["metrics"]`,
		"malformed":         `{"type":"metrics",`,
		"cpu above 100":     `{"type":"metrics","ts":1,"mono_ms":2,"cpu_percent":100.1}`,
		"cpu below zero":    `{"type":"metrics","ts":1,"mono_ms":2,"cpu_percent":-0.5}`,
		"mem used > total":  `{"type":"metrics","ts":1,"mono_ms":2,"mem_total":10,"mem_used":11}`,
		"mem half":          `{"type":"metrics","ts":1,"mono_ms":2,"mem_total":10}`,
		"swap half":         `{"type":"metrics","ts":1,"mono_ms":2,"swap_used":1}`,
		"disk partial":      `{"type":"metrics","ts":1,"mono_ms":2,"disk_total":10,"disk_used":1}`,
		"disk used > total": `{"type":"metrics","ts":1,"mono_ms":2,"disk_total":10,"disk_used":11,"disk_free":0}`,
		"disk free > total": `{"type":"metrics","ts":1,"mono_ms":2,"disk_total":10,"disk_used":1,"disk_free":11}`,
		"load partial":      `{"type":"metrics","ts":1,"mono_ms":2,"load1":0.1,"load5":0.1}`,
		"negative load":     `{"type":"metrics","ts":1,"mono_ms":2,"load1":-0.1,"load5":0.1,"load15":0.1}`,
		"iface empty name":  `{"type":"metrics","ts":1,"mono_ms":2,"net":{"":{"rx":1,"tx":1}}}`,
		"iface with space":  `{"type":"metrics","ts":1,"mono_ms":2,"net":{"eth 0":{"rx":1,"tx":1}}}`,
		"iface control":     `{"type":"metrics","ts":1,"mono_ms":2,"net":{"eth\u00000":{"rx":1,"tx":1}}}`,
		"iface unknown key": `{"type":"metrics","ts":1,"mono_ms":2,"net":{"eth0":{"rx":1,"tx":1,"z":1}}}`,
		"cores negative":    `{"type":"host_info","cpu_cores":-1,"interval_ms":5000}`,
		"cores too many":    `{"type":"host_info","cpu_cores":4097,"interval_ms":5000}`,
		"interval too low":  `{"type":"host_info","cpu_cores":1,"interval_ms":2999}`,
		"interval too high": `{"type":"host_info","cpu_cores":1,"interval_ms":60001}`,
		"interval missing":  `{"type":"host_info","cpu_cores":1}`,
	}
	for name, in := range cases {
		if _, err := DecodeAgentMessage([]byte(in)); err == nil {
			t.Errorf("%s: decoded without error", name)
		}
	}
}

func TestIfaceCountLimit(t *testing.T) {
	build := func(n int) []byte {
		var b strings.Builder
		b.WriteString(`{"type":"metrics","ts":1,"mono_ms":2,"net":{`)
		for i := range n {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`"eth` + strconv.Itoa(i) + `":{"rx":1,"tx":1}`)
		}
		b.WriteString(`}}`)
		return []byte(b.String())
	}
	if _, err := DecodeAgentMessage(build(MaxInterfaces)); err != nil {
		t.Errorf("%d interfaces: %v", MaxInterfaces, err)
	}
	if _, err := DecodeAgentMessage(build(MaxInterfaces + 1)); err == nil {
		t.Errorf("%d interfaces: decoded without error", MaxInterfaces+1)
	}
}

func TestIfaceNameTooLong(t *testing.T) {
	name := strings.Repeat("e", maxIfaceNameLen)
	ok := `{"type":"metrics","ts":1,"mono_ms":2,"net":{"` + name + `":{"rx":1,"tx":1}}}`
	if _, err := DecodeAgentMessage([]byte(ok)); err != nil {
		t.Errorf("name of %d characters: %v", maxIfaceNameLen, err)
	}
	long := `{"type":"metrics","ts":1,"mono_ms":2,"net":{"` + name + `e":{"rx":1,"tx":1}}}`
	if _, err := DecodeAgentMessage([]byte(long)); err == nil {
		t.Error("over-long interface name decoded without error")
	}
}

// Hostile strings are kept but sanitized: the panel renders them as text.
func TestHostInfoNormalize(t *testing.T) {
	long := strings.Repeat("a", maxStringLen+10)
	in := `{"type":"host_info","hostname":"<script>x\u0000\ny</script>","os":"` + long +
		`","cpu_cores":1,"interval_ms":5000}`
	m, err := DecodeAgentMessage([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	info := m.(*HostInfo)
	if info.Hostname != "<script>xy</script>" {
		t.Errorf("hostname = %q", info.Hostname)
	}
	if len([]rune(info.OS)) != maxStringLen {
		t.Errorf("os length = %d, want %d", len([]rune(info.OS)), maxStringLen)
	}
}

func TestSizeLimits(t *testing.T) {
	pad := func(n int) []byte {
		return []byte(`{"type":"metrics","ts":1,"mono_ms":2,"cpu_model":"` +
			strings.Repeat("a", n) + `"}`)
	}
	if _, err := DecodeAgentMessage(pad(MaxPanelRead)); err != ErrTooLarge {
		t.Errorf("panel: err = %v, want ErrTooLarge", err)
	}
	if _, err := DecodePanelMessage(pad(MaxAgentRead)); err != ErrTooLarge {
		t.Errorf("agent: err = %v, want ErrTooLarge", err)
	}
	// Just under the agent limit, the message is parsed rather than rejected
	// for its size, so it fails as an unknown type instead.
	if _, err := DecodePanelMessage([]byte(`{"type":"metrics"}`)); err == ErrTooLarge {
		t.Error("small message reported as too large")
	}
}

func TestDecodePanelMessage(t *testing.T) {
	m, err := DecodePanelMessage([]byte(`{"type":"registered","agent_id":"h1","secret":"s3cret"}`))
	if err != nil {
		t.Fatal(err)
	}
	reg, ok := m.(*Registered)
	if !ok {
		t.Fatalf("got %T, want *Registered", m)
	}
	if reg.AgentID != "h1" || reg.Secret != "s3cret" {
		t.Errorf("registered = %+v", reg)
	}

	e, err := DecodePanelMessage([]byte(`{"type":"error","code":"unauthorized","message":"bad\ttoken"}`))
	if err != nil {
		t.Fatal(err)
	}
	em := e.(*ErrorMessage)
	if em.Code != CodeUnauthorized || em.Message != "badtoken" {
		t.Errorf("error = %+v", em)
	}

	// An unknown code is accepted so a newer panel does not break the agent.
	if _, err := DecodePanelMessage([]byte(`{"type":"error","code":"future_code","message":""}`)); err != nil {
		t.Errorf("unknown code: %v", err)
	}
}

func TestDecodePanelMessageRejects(t *testing.T) {
	cases := map[string]string{
		"unknown type":   `{"type":"exec","cmd":"sh"}`,
		"agent type":     `{"type":"metrics","ts":1,"mono_ms":2}`,
		"unknown field":  `{"type":"registered","agent_id":"a","secret":"b","cmd":"sh"}`,
		"empty agent_id": `{"type":"registered","agent_id":"","secret":"b"}`,
		"empty secret":   `{"type":"registered","agent_id":"a","secret":""}`,
		"dot in id":      `{"type":"registered","agent_id":"a.b","secret":"c"}`,
		"dot in secret":  `{"type":"registered","agent_id":"a","secret":"b.c"}`,
		"space in id":    `{"type":"registered","agent_id":"a b","secret":"c"}`,
		"empty code":     `{"type":"error","code":"","message":"x"}`,
		"upper code":     `{"type":"error","code":"Unauthorized","message":"x"}`,
		"trailing data":  `{"type":"error","code":"x","message":"y"}{}`,
	}
	for name, in := range cases {
		if _, err := DecodePanelMessage([]byte(in)); err == nil {
			t.Errorf("%s: decoded without error", name)
		}
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	cpu := 3.5
	in := &Metrics{TS: 7, MonoMS: 8, CPUPercent: &cpu,
		Net: map[string]NetCounters{"eth0": {RX: 1, TX: 2}}}
	data, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeAgentMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	got := out.(*Metrics)
	if got.Type != TypeMetrics || got.TS != 7 || *got.CPUPercent != 3.5 || got.MemTotal != nil {
		t.Errorf("round trip = %+v", got)
	}

	reg := &Registered{AgentID: "h1", Secret: "s1"}
	data, err = Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePanelMessage(data); err != nil {
		t.Fatal(err)
	}
	if _, err := Marshal(struct{}{}); err == nil {
		t.Error("marshalled an unsupported type")
	}
}

// Byte counters are bounded so that a hostile agent cannot overflow the
// INTEGER columns they are stored in (REQUIREMENTS 5.4).
func TestByteCountersAreBounded(t *testing.T) {
	cases := map[string]string{
		"mem":    `{"type":"metrics","ts":1,"mono_ms":2,"mem_total":18446744073709551615,"mem_used":0}`,
		"swap":   `{"type":"metrics","ts":1,"mono_ms":2,"swap_total":1125899906842625,"swap_used":0}`,
		"disk":   `{"type":"metrics","ts":1,"mono_ms":2,"disk_total":1125899906842625,"disk_used":0,"disk_free":0}`,
		"uptime": `{"type":"metrics","ts":1,"mono_ms":2,"uptime":3153600001}`,
	}
	for name, in := range cases {
		if _, err := DecodeAgentMessage([]byte(in)); err == nil {
			t.Errorf("%s: decoded without error", name)
		}
	}
	ok := `{"type":"metrics","ts":1,"mono_ms":2,"mem_total":1125899906842624,"mem_used":1,"uptime":3153600000}`
	if _, err := DecodeAgentMessage([]byte(ok)); err != nil {
		t.Errorf("values at the limit rejected: %v", err)
	}
}

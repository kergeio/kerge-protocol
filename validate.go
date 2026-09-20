package protocol

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

// Limits applied to untrusted input (REQUIREMENTS 5.4).
const (
	maxStringLen     = 128
	maxMessageLen    = 256
	maxCodeLen       = 32
	maxCredentialLen = 128
	maxIfaceNameLen  = 32
	maxCPUCores      = 4096
	// maxBytes bounds memory, swap and disk figures. A petabyte is far
	// beyond any monitored host and keeps the values inside the INTEGER
	// columns they are stored in.
	maxBytes = 1 << 50
	// maxUptime is a hundred years in seconds.
	maxUptime     = 100 * 365 * 24 * 60 * 60
	minIntervalMS = 3000
	maxIntervalMS = 60000
)

// Normalize strips control characters from the string fields and truncates
// them to maxStringLen characters.
func (m *HostInfo) Normalize() {
	m.Hostname = sanitize(m.Hostname, maxStringLen)
	m.OS = sanitize(m.OS, maxStringLen)
	m.Platform = sanitize(m.Platform, maxStringLen)
	m.PlatformVersion = sanitize(m.PlatformVersion, maxStringLen)
	m.Kernel = sanitize(m.Kernel, maxStringLen)
	m.Arch = sanitize(m.Arch, maxStringLen)
	m.CPUModel = sanitize(m.CPUModel, maxStringLen)
	m.AgentVersion = sanitize(m.AgentVersion, maxStringLen)
}

// Validate checks the numeric fields. Call Normalize first.
func (m *HostInfo) Validate() error {
	if m.CPUCores < 0 || m.CPUCores > maxCPUCores {
		return fmt.Errorf("protocol: cpu_cores out of range: %d", m.CPUCores)
	}
	if m.IntervalMS < minIntervalMS || m.IntervalMS > maxIntervalMS {
		return fmt.Errorf("protocol: interval_ms out of range: %d", m.IntervalMS)
	}
	return nil
}

// Validate rejects a sample with an impossible value. A collector that timed
// out omits its whole group of fields, so each group must be either complete
// or absent.
func (m *Metrics) Validate() error {
	if m.TS < 0 {
		return fmt.Errorf("protocol: ts out of range: %d", m.TS)
	}
	if m.MonoMS < 0 {
		return fmt.Errorf("protocol: mono_ms out of range: %d", m.MonoMS)
	}
	if m.CPUPercent != nil {
		if err := checkFloat("cpu_percent", *m.CPUPercent, 0, 100); err != nil {
			return err
		}
	}
	if m.Uptime != nil && *m.Uptime > maxUptime {
		return fmt.Errorf("protocol: uptime out of range: %d", *m.Uptime)
	}
	if err := checkPair("mem_total", m.MemTotal, "mem_used", m.MemUsed); err != nil {
		return err
	}
	if err := checkPair("swap_total", m.SwapTotal, "swap_used", m.SwapUsed); err != nil {
		return err
	}
	if present(m.DiskTotal) != present(m.DiskUsed) || present(m.DiskTotal) != present(m.DiskFree) {
		return errors.New("protocol: disk_total, disk_used and disk_free must be reported together")
	}
	if m.DiskTotal != nil {
		if *m.DiskTotal > maxBytes {
			return fmt.Errorf("protocol: disk_total out of range: %d", *m.DiskTotal)
		}
		if *m.DiskUsed > *m.DiskTotal {
			return errors.New("protocol: disk_used exceeds disk_total")
		}
		if *m.DiskFree > *m.DiskTotal {
			return errors.New("protocol: disk_free exceeds disk_total")
		}
	}
	if present(m.Load1) != present(m.Load5) || present(m.Load1) != present(m.Load15) {
		return errors.New("protocol: load1, load5 and load15 must be reported together")
	}
	if m.Load1 != nil {
		for _, l := range []struct {
			name string
			v    float64
		}{{"load1", *m.Load1}, {"load5", *m.Load5}, {"load15", *m.Load15}} {
			if err := checkFloat(l.name, l.v, 0, math.MaxFloat64); err != nil {
				return err
			}
		}
	}
	if len(m.Net) > MaxInterfaces {
		return fmt.Errorf("protocol: too many interfaces: %d", len(m.Net))
	}
	for name := range m.Net {
		if !validIfaceName(name) {
			return fmt.Errorf("protocol: invalid interface name %q", clip(name))
		}
	}
	return nil
}

// Validate checks the long-lived credential. Neither part may contain a dot,
// because the two are joined with one in the Authorization header.
func (m *Registered) Validate() error {
	if err := checkCredential("agent_id", m.AgentID); err != nil {
		return err
	}
	return checkCredential("secret", m.Secret)
}

// Normalize strips control characters from the human-readable message.
func (m *ErrorMessage) Normalize() {
	m.Message = sanitize(m.Message, maxMessageLen)
}

// Validate checks the shape of the code. Unknown codes are accepted so that
// a newer panel can add one without breaking older agents.
func (m *ErrorMessage) Validate() error {
	if m.Code == "" || len(m.Code) > maxCodeLen {
		return fmt.Errorf("protocol: invalid error code %q", clip(m.Code))
	}
	for _, c := range []byte(m.Code) {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return fmt.Errorf("protocol: invalid error code %q", clip(m.Code))
		}
	}
	return nil
}

func present[T any](p *T) bool { return p != nil }

// checkPair verifies that a total/used pair is complete, consistent and
// within range.
func checkPair(totalName string, total *uint64, usedName string, used *uint64) error {
	if present(total) != present(used) {
		return fmt.Errorf("protocol: %s and %s must be reported together", totalName, usedName)
	}
	if total == nil {
		return nil
	}
	if *total > maxBytes {
		return fmt.Errorf("protocol: %s out of range: %d", totalName, *total)
	}
	if *used > *total {
		return fmt.Errorf("protocol: %s exceeds %s", usedName, totalName)
	}
	return nil
}

func checkFloat(name string, v, min, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return fmt.Errorf("protocol: %s out of range: %v", name, v)
	}
	return nil
}

func checkCredential(name, v string) error {
	if v == "" || len(v) > maxCredentialLen {
		return fmt.Errorf("protocol: invalid %s", name)
	}
	for _, c := range []byte(v) {
		if c <= ' ' || c > '~' || c == '.' {
			return fmt.Errorf("protocol: invalid %s", name)
		}
	}
	return nil
}

// validIfaceName allows printable ASCII without spaces, which covers every
// interface name a kernel will produce.
func validIfaceName(name string) bool {
	if name == "" || len(name) > maxIfaceNameLen {
		return false
	}
	for _, c := range []byte(name) {
		if c <= ' ' || c > '~' {
			return false
		}
	}
	return true
}

// sanitize drops control characters and keeps at most max characters.
func sanitize(s string, max int) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		if n == max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// clip shortens a value for an error message.
func clip(s string) string {
	if len(s) > maxCodeLen {
		return s[:maxCodeLen]
	}
	return s
}

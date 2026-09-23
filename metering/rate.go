// Package metering turns the cumulative counters an agent reports into the
// figures a panel stores: network rates (REQUIREMENTS 3.1.4) and the
// traffic carried since the previous readings (REQUIREMENTS 3.4).
//
// It lives beside the protocol because it defines how the reported
// counters are to be read, which every panel implementation must agree on.
package metering

import (
	"sync"
	"time"

	"github.com/kergeio/kerge-protocol"
	"github.com/kergeio/kerge-protocol/ifacefilter"
)

// maxClockSkew is how far the agent's monotonic delta may differ from the
// receiver's own clock before a sample is considered unusable for a rate:
// either one may be at most this many times the other.
const maxClockSkew = 2

// RateTracker computes network rates from consecutive metrics messages.
// K identifies the host the samples belong to. A tracker is safe for
// concurrent use.
type RateTracker[K comparable] struct {
	mu   sync.Mutex
	last map[K]*reading
}

// reading is the previous sample a rate is computed against.
type reading struct {
	monoMS   int64
	received time.Time
	counters map[string]protocol.NetCounters
}

// NewRateTracker returns an empty tracker.
func NewRateTracker[K comparable]() *RateTracker[K] {
	return &RateTracker[K]{last: make(map[K]*reading)}
}

// Rate returns the combined receive and transmit rate in bytes per second
// over the interfaces of m that exclude does not match, and records m as
// the new baseline for host.
//
// ok is false when no rate can be computed, in which case the caller
// records no rate at all rather than zero:
//
//   - the first sample of a host has nothing to compare against;
//   - the agent's monotonic clock did not advance, which means a restarted
//     agent or a repeated sample;
//   - the monotonic delta and the receiver's own elapsed time disagree by
//     more than a factor of maxClockSkew, which makes the sample
//     untrustworthy for a rate.
//
// An interface whose counter went backwards, typically after the host
// rebooted, contributes nothing this cycle; an interface seen for the
// first time only establishes a baseline.
func (t *RateTracker[K]) Rate(host K, m *protocol.Metrics, received time.Time, exclude []string) (rx, tx float64, ok bool) {
	counters := make(map[string]protocol.NetCounters, len(m.Net))
	for name, c := range m.Net {
		// The agent already filtered by its own rules; the panel applies
		// its own on top (REQUIREMENTS 3.1.3).
		if !ifacefilter.Match(exclude, name) {
			counters[name] = c
		}
	}

	t.mu.Lock()
	previous := t.last[host]
	t.last[host] = &reading{monoMS: m.MonoMS, received: received, counters: counters}
	t.mu.Unlock()

	if previous == nil {
		return 0, 0, false
	}
	deltaMono := m.MonoMS - previous.monoMS
	deltaReceived := received.Sub(previous.received).Milliseconds()
	if deltaMono <= 0 || deltaMono > maxClockSkew*deltaReceived || deltaReceived > maxClockSkew*deltaMono {
		return 0, 0, false
	}

	seconds := float64(deltaMono) / 1000
	for name, current := range counters {
		before, seen := previous.counters[name]
		if !seen || current.RX < before.RX || current.TX < before.TX {
			continue
		}
		rx += float64(current.RX-before.RX) / seconds
		tx += float64(current.TX-before.TX) / seconds
	}
	return rx, tx, true
}

// Forget drops the baseline of a host, for instance when it is deleted.
func (t *RateTracker[K]) Forget(host K) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, host)
}

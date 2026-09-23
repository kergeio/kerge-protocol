package metering

import (
	"math"

	"github.com/kergeio/kerge-protocol"
	"github.com/kergeio/kerge-protocol/ifacefilter"
)

// Traffic returns the bytes received and transmitted since the previous
// readings, summed over the interfaces of now that exclude does not match
// (REQUIREMENTS 3.4).
//
// previous holds the last counters of every interface of the host seen so
// far, including interfaces that have since disappeared: the caller keeps
// them, so a returning interface is measured against its saved value. Per
// interface and per counter:
//
//   - an interface missing from previous is new, typically on the host's
//     first report, and only establishes a baseline: it adds nothing;
//   - a counter that went backwards was reset, typically by a reboot, so
//     everything it counts now is new traffic: it adds its current value;
//   - otherwise it adds the difference.
//
// No case yields a negative amount, and the sums stop at the largest
// uint64 rather than wrapping around, whatever the agent reports. After
// the call the caller replaces the saved counters of every interface in
// now, excluded or not, so that changing the exclusion rules later does
// not count old traffic.
func Traffic(previous, now map[string]protocol.NetCounters, exclude []string) (rx, tx uint64) {
	for name, current := range now {
		if ifacefilter.Match(exclude, name) {
			continue
		}
		before, seen := previous[name]
		if !seen {
			continue
		}
		rx = saturatingAdd(rx, counterDelta(before.RX, current.RX))
		tx = saturatingAdd(tx, counterDelta(before.TX, current.TX))
	}
	return rx, tx
}

// saturatingAdd returns a+b, or the largest uint64 if that overflows.
func saturatingAdd(a, b uint64) uint64 {
	if a > math.MaxUint64-b {
		return math.MaxUint64
	}
	return a + b
}

// counterDelta is what one cumulative counter carried since before.
func counterDelta(before, now uint64) uint64 {
	if now < before {
		return now
	}
	return now - before
}

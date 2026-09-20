package metering

import (
	"testing"
	"time"

	"kerge.io/protocol"
)

func sample(monoMS int64, counters map[string]protocol.NetCounters) *protocol.Metrics {
	return &protocol.Metrics{Type: protocol.TypeMetrics, TS: 1, MonoMS: monoMS, Net: counters}
}

func eth(rx, tx uint64) map[string]protocol.NetCounters {
	return map[string]protocol.NetCounters{"eth0": {RX: rx, TX: tx}}
}

var start = time.Unix(1_700_000_000, 0)

func TestRate(t *testing.T) {
	tr := NewRateTracker[int64]()
	if _, _, ok := tr.Rate(1, sample(1000, eth(1000, 2000)), start, nil); ok {
		t.Fatal("the first sample produced a rate")
	}
	rx, tx, ok := tr.Rate(1, sample(6000, eth(6000, 9000)), start.Add(5*time.Second), nil)
	if !ok {
		t.Fatal("the second sample produced no rate")
	}
	if rx != 1000 || tx != 1400 {
		t.Errorf("rx=%v tx=%v, want 1000 and 1400 bytes per second", rx, tx)
	}
}

// Hosts are tracked independently.
func TestRateIsPerHost(t *testing.T) {
	tr := NewRateTracker[string]()
	tr.Rate("a", sample(1000, eth(0, 0)), start, nil)
	if _, _, ok := tr.Rate("b", sample(2000, eth(500, 500)), start.Add(time.Second), nil); ok {
		t.Error("host b borrowed the baseline of host a")
	}
	if rx, _, ok := tr.Rate("a", sample(2000, eth(500, 500)), start.Add(time.Second), nil); !ok || rx != 500 {
		t.Errorf("host a: rx=%v ok=%v, want 500", rx, ok)
	}
}

// Rates are only reported when the two clocks agree (REQUIREMENTS 3.1.4).
func TestRateNeedsTrustworthyClocks(t *testing.T) {
	cases := map[string]struct {
		elapsed time.Duration
		monoMS  int64
	}{
		"monotonic clock went backwards":             {5 * time.Second, 500},
		"monotonic clock did not advance":            {5 * time.Second, 1000},
		"monotonic delta far above the elapsed time": {time.Second, 60000},
		"elapsed time far above the monotonic delta": {time.Minute, 2000},
	}
	for name, c := range cases {
		tr := NewRateTracker[int64]()
		tr.Rate(1, sample(1000, eth(1000, 2000)), start, nil)
		if _, _, ok := tr.Rate(1, sample(c.monoMS, eth(6000, 9000)), start.Add(c.elapsed), nil); ok {
			t.Errorf("%s: a rate was reported", name)
		}
	}
	// Exactly at the limit the sample is still usable.
	tr := NewRateTracker[int64]()
	tr.Rate(1, sample(0, eth(0, 0)), start, nil)
	if _, _, ok := tr.Rate(1, sample(2000, eth(0, 0)), start.Add(time.Second), nil); !ok {
		t.Error("a delta of exactly twice the elapsed time was rejected")
	}
}

// A counter that went backwards means the interface contributes nothing,
// but the other interfaces still count.
func TestCounterWentBackwards(t *testing.T) {
	two := func(a, b uint64) map[string]protocol.NetCounters {
		return map[string]protocol.NetCounters{"eth0": {RX: a, TX: a}, "eth1": {RX: b, TX: b}}
	}
	tr := NewRateTracker[int64]()
	tr.Rate(1, sample(1000, two(1000, 1000)), start, nil)
	rx, tx, ok := tr.Rate(1, sample(2000, two(10, 3000)), start.Add(time.Second), nil)
	if !ok || rx != 2000 || tx != 2000 {
		t.Errorf("rx=%v tx=%v ok=%v, want 2000 from eth1 alone", rx, tx, ok)
	}
}

// An interface seen for the first time only establishes a baseline.
func TestNewInterfaceOnlyEstablishesABaseline(t *testing.T) {
	tr := NewRateTracker[int64]()
	tr.Rate(1, sample(1000, nil), start, nil)
	if rx, _, ok := tr.Rate(1, sample(2000, eth(5000, 5000)), start.Add(time.Second), nil); !ok || rx != 0 {
		t.Errorf("rx=%v ok=%v, want 0", rx, ok)
	}
	if rx, _, ok := tr.Rate(1, sample(3000, eth(6000, 6000)), start.Add(2*time.Second), nil); !ok || rx != 1000 {
		t.Errorf("rx=%v ok=%v, want 1000 once a baseline exists", rx, ok)
	}
}

// Excluded interfaces are left out of the sum (REQUIREMENTS 3.1.3).
func TestExcludedInterfaces(t *testing.T) {
	counters := func(a, b uint64) map[string]protocol.NetCounters {
		return map[string]protocol.NetCounters{"eth0": {RX: a, TX: a}, "docker0": {RX: b, TX: b}}
	}
	tr := NewRateTracker[int64]()
	exclude := []string{"docker*"}
	tr.Rate(1, sample(1000, counters(0, 0)), start, exclude)
	if rx, _, ok := tr.Rate(1, sample(2000, counters(100, 5000)), start.Add(time.Second), exclude); !ok || rx != 100 {
		t.Errorf("rx=%v, want only eth0 counted", rx)
	}
}

func TestForget(t *testing.T) {
	tr := NewRateTracker[int64]()
	tr.Rate(1, sample(1000, eth(0, 0)), start, nil)
	tr.Forget(1)
	if _, _, ok := tr.Rate(1, sample(2000, eth(500, 500)), start.Add(time.Second), nil); ok {
		t.Error("the baseline survived Forget")
	}
	if len(tr.last) != 1 {
		t.Errorf("tracker holds %d entries, want the fresh baseline only", len(tr.last))
	}
}

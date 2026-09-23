package metering

import (
	"testing"

	"github.com/kergeio/kerge-protocol"
)

type counters = map[string]protocol.NetCounters

func TestTraffic(t *testing.T) {
	cases := []struct {
		name     string
		previous counters
		now      counters
		exclude  []string
		rx, tx   uint64
	}{
		{
			name:     "difference since the previous reading",
			previous: counters{"eth0": {RX: 1000, TX: 2000}},
			now:      counters{"eth0": {RX: 1500, TX: 2700}},
			rx:       500, tx: 700,
		},
		{
			name:     "unchanged counters add nothing",
			previous: counters{"eth0": {RX: 1000, TX: 2000}},
			now:      counters{"eth0": {RX: 1000, TX: 2000}},
		},
		{
			name: "the first report only establishes a baseline",
			now:  counters{"eth0": {RX: 9_000_000, TX: 8_000_000}},
		},
		{
			name:     "a new interface only establishes a baseline",
			previous: counters{"eth0": {RX: 1000, TX: 1000}},
			now:      counters{"eth0": {RX: 1100, TX: 1200}, "eth1": {RX: 5000, TX: 5000}},
			rx:       100, tx: 200,
		},
		{
			name:     "a reset counter counts its new value",
			previous: counters{"eth0": {RX: 9_000_000, TX: 8_000_000}},
			now:      counters{"eth0": {RX: 300, TX: 400}},
			rx:       300, tx: 400,
		},
		{
			name:     "counters are judged one by one",
			previous: counters{"eth0": {RX: 9_000_000, TX: 1000}},
			now:      counters{"eth0": {RX: 300, TX: 1500}},
			rx:       300, tx: 500,
		},
		{
			name: "a returning interface is measured against its saved value",
			previous: counters{
				"eth0": {RX: 1000, TX: 1000},
				"eth1": {RX: 7000, TX: 7000}, // missing from the last reports
			},
			now: counters{"eth0": {RX: 1000, TX: 1000}, "eth1": {RX: 7500, TX: 7100}},
			rx:  500, tx: 100,
		},
		{
			name:     "a vanished interface adds nothing",
			previous: counters{"eth0": {RX: 1000, TX: 1000}, "eth1": {RX: 7000, TX: 7000}},
			now:      counters{"eth0": {RX: 1200, TX: 1300}},
			rx:       200, tx: 300,
		},
		{
			name:     "excluded interfaces are left out of the sum",
			previous: counters{"eth0": {RX: 0, TX: 0}, "docker0": {RX: 0, TX: 0}},
			now:      counters{"eth0": {RX: 10, TX: 20}, "docker0": {RX: 5000, TX: 5000}},
			exclude:  []string{"docker*"},
			rx:       10, tx: 20,
		},
		{
			name:     "interfaces add up",
			previous: counters{"eth0": {RX: 0, TX: 0}, "eth1": {RX: 100, TX: 100}},
			now:      counters{"eth0": {RX: 10, TX: 20}, "eth1": {RX: 130, TX: 140}},
			rx:       40, tx: 60,
		},
		{
			name:     "sums stop at the largest value instead of wrapping around",
			previous: counters{"eth0": {RX: 0, TX: 0}, "eth1": {RX: 0, TX: 0}},
			now:      counters{"eth0": {RX: 1<<64 - 1, TX: 10}, "eth1": {RX: 1<<64 - 1, TX: 20}},
			rx:       1<<64 - 1, tx: 30,
		},
		{
			name:     "the largest counter values do not overflow one interface",
			previous: counters{"eth0": {RX: 1<<64 - 1000, TX: 0}},
			now:      counters{"eth0": {RX: 1<<64 - 1, TX: 1<<64 - 1}},
			rx:       999, tx: 1<<64 - 1,
		},
	}
	for _, c := range cases {
		rx, tx := Traffic(c.previous, c.now, c.exclude)
		if rx != c.rx || tx != c.tx {
			t.Errorf("%s: rx=%d tx=%d, want %d and %d", c.name, rx, tx, c.rx, c.tx)
		}
	}
}

// A host that reboots between two readings never loses traffic it already
// counted and never counts a negative amount (REQUIREMENTS 3.4, 12).
func TestTrafficAcrossAReboot(t *testing.T) {
	readings := []counters{
		{"eth0": {RX: 1000, TX: 1000}},
		{"eth0": {RX: 4000, TX: 3000}},
		{"eth0": {RX: 200, TX: 100}}, // rebooted
		{"eth0": {RX: 700, TX: 900}},
	}
	var totalRX, totalTX uint64
	for i := 1; i < len(readings); i++ {
		rx, tx := Traffic(readings[i-1], readings[i], nil)
		totalRX += rx
		totalTX += tx
	}
	if totalRX != 3000+200+500 || totalTX != 2000+100+800 {
		t.Errorf("rx=%d tx=%d, want 3700 and 2900", totalRX, totalTX)
	}
}

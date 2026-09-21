package main

import (
	"path/filepath"
	"testing"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// stubTrafficSource stands in for the SSH engine: live byte counters can only be
// produced by a real forwarding connection, and drainTraffic reads them through
// this one seam.
type stubTrafficSource map[string][2]uint64

func (s stubTrafficSource) Traffic(tunnelID string) (uint64, uint64) {
	counters := s[tunnelID]
	return counters[0], counters[1]
}

func newTrafficTestDB(t *testing.T) *db.DB {
	t.Helper()

	d, err := db.Open(filepath.Join(t.TempDir(), "eazy-gateway.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func seedTunnel(t *testing.T, d *db.DB) string {
	t.Helper()

	id, err := d.CreateTunnel(&db.TunnelConfig{Name: "edge", Type: db.TunnelTypeLocal, ListenPort: 9000})
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}
	return id
}

// A tunnel that was forwarding when the process exited must not lose the bytes
// it moved since the last persisted snapshot: they are folded into the totals
// before the engine stops and the runtime counters reset.
func TestDrainTrafficFoldsRuntimeCountersIntoPersistedTotals(t *testing.T) {
	d := newTrafficTestDB(t)
	id := seedTunnel(t, d)

	if err := d.UpdateTraffic(id, &db.TrafficStats{TotalBytesIn: 500, TotalBytesOut: 700}); err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	if err := drainTraffic(d, stubTrafficSource{id: {1024, 2048}}); err != nil {
		t.Fatalf("drainTraffic: %v", err)
	}

	got, err := d.GetTraffic(id)
	if err != nil {
		t.Fatalf("get traffic: %v", err)
	}
	if got.TotalBytesIn != 1524 || got.TotalBytesOut != 2748 {
		t.Errorf("traffic = %d/%d, want 1524/2748", got.TotalBytesIn, got.TotalBytesOut)
	}
}

// Tunnels that are not running report zero counters, and draining must leave
// them completely alone: an idle tunnel neither gains a traffic record nor has
// its persisted totals rewritten.
func TestDrainTrafficLeavesStoppedTunnelsUntouched(t *testing.T) {
	d := newTrafficTestDB(t)
	id := seedTunnel(t, d)

	if err := drainTraffic(d, stubTrafficSource{}); err != nil {
		t.Fatalf("drainTraffic (no record yet): %v", err)
	}
	if got, err := d.GetTraffic(id); err != nil {
		t.Fatalf("get traffic: %v", err)
	} else if got != nil {
		t.Errorf("idle tunnel gained a traffic record: %+v", got)
	}

	if err := d.UpdateTraffic(id, &db.TrafficStats{TotalBytesIn: 100, TotalBytesOut: 200}); err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	if err := drainTraffic(d, stubTrafficSource{}); err != nil {
		t.Fatalf("drainTraffic (record present): %v", err)
	}
	got, err := d.GetTraffic(id)
	if err != nil {
		t.Fatalf("get traffic: %v", err)
	}
	if got.TotalBytesIn != 100 || got.TotalBytesOut != 200 {
		t.Errorf("traffic = %d/%d, want 100/200", got.TotalBytesIn, got.TotalBytesOut)
	}
}

package db

import (
	"path/filepath"
	"testing"
)

func newTrafficDB(t *testing.T) *DB {
	t.Helper()

	d, err := Open(filepath.Join(t.TempDir(), "eazy-gateway.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// AddTraffic is the one place runtime counters are folded into the persisted
// totals. A tunnel that has never been recorded starts from zero, and a tunnel
// that already has totals has them increased rather than replaced.
func TestAddTrafficFoldsCountersIntoPersistedTotals(t *testing.T) {
	d := newTrafficDB(t)

	if err := d.AddTraffic("fresh", 10, 20); err != nil {
		t.Fatalf("AddTraffic (no record yet): %v", err)
	}
	first, err := d.GetTraffic("fresh")
	if err != nil {
		t.Fatalf("GetTraffic: %v", err)
	}
	if first.TotalBytesIn != 10 || first.TotalBytesOut != 20 {
		t.Fatalf("traffic after first add = %d/%d, want 10/20", first.TotalBytesIn, first.TotalBytesOut)
	}

	if err := d.AddTraffic("fresh", 1024, 2048); err != nil {
		t.Fatalf("AddTraffic (record present): %v", err)
	}
	second, err := d.GetTraffic("fresh")
	if err != nil {
		t.Fatalf("GetTraffic: %v", err)
	}
	if second.TotalBytesIn != 1034 || second.TotalBytesOut != 2068 {
		t.Errorf("traffic after second add = %d/%d, want 1034/2068", second.TotalBytesIn, second.TotalBytesOut)
	}
}

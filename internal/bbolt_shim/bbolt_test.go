package bbolt_shim

import (
	"os"
	"path/filepath"
	"testing"
)

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

// The shim stores every bucket as readable JSON, so the db file carries the
// admin password hash plus every host and tunnel definition in the clear. It
// must honour the mode the caller asked for rather than hardcoding 0644.
func TestOpenHonoursRequestedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")

	d, err := Open(path, 0o600, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := d.Update(func(tx *Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("hosts"))
		if err != nil {
			return err
		}
		return b.Put([]byte("h1"), []byte("host-value"))
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("db mode = %o, want 600", got)
	}
}

// A db written by an older version is already 0644 on disk; opening it has to
// tighten it, otherwise upgrading leaves the credentials readable forever.
func TestOpenRepairsExistingLooseMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	d, err := Open(path, 0o600, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("db mode after open = %o, want 600", got)
	}
}

// A zero mode would create an unreadable 0000 file; fall back to owner-only.
func TestOpenWithoutModeIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")

	d, err := Open(path, 0, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("db mode = %o, want 600 when the caller passes no mode", got)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
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

// Everything the daemon writes lands in this file, and older versions also put
// the generated admin password in it, so it must stay owner only.
func TestOpenLogFileIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eazy-gateway.log")

	f, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	f.Close()

	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("log mode = %o, want 600", got)
	}
}

// Installing over an older version must tighten the log it already created.
func TestOpenLogFileRepairsLooseMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eazy-gateway.log")
	if err := os.WriteFile(path, []byte("{\"msg\":\"old\"}\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	f, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	f.Close()

	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("log mode = %o, want 600 after opening an existing log", got)
	}
}

// The generated password now lives in this file alone, since the log no longer
// carries it, so the file must be owner only on creation and on rewrite.
func TestWriteInitialPasswordIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "initial-password.txt")

	if err := writeInitialPassword(path, "generated-password"); err != nil {
		t.Fatalf("writeInitialPassword: %v", err)
	}
	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("password file mode = %o, want 600", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.TrimRight(string(data), "\n") != "generated-password" {
		t.Errorf("password file content = %q", data)
	}

	// A file left world readable by an older version must be tightened.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := writeInitialPassword(path, "rotated-password"); err != nil {
		t.Fatalf("writeInitialPassword (rewrite): %v", err)
	}
	if got := modeOf(t, path); got != 0o600 {
		t.Errorf("password file mode after rewrite = %o, want 600", got)
	}
}

// The daemon appends across restarts; reopening must not truncate history.
func TestOpenLogFileAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eazy-gateway.log")

	first, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	if _, err := first.WriteString("first run\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	first.Close()

	second, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	if _, err := second.WriteString("second run\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	second.Close()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, want := range []string{"first run", "second run"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("log %q missing %q", got, want)
		}
	}
}

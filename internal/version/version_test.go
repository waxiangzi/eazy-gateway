package version

import (
	"strings"
	"testing"
)

// Tests compile without -ldflags, so Version is the un-injected default here.
// It must still be usable: an empty value would render a blank version row in
// the console and a bare newline from `eazy-gateway --version`.
func TestDefaultVersionIsUsable(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must never be empty")
	}
	if strings.TrimSpace(Version) != Version {
		t.Fatalf("Version %q must not be padded with whitespace", Version)
	}
	if strings.ContainsAny(Version, "\r\n") {
		t.Fatalf("Version %q must be a single line", Version)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
	"github.com/eazy-gateway/eazy-gateway/internal/version"
)

// newSettingsTestEnv opens a database over a fresh temporary data directory and
// returns the settings endpoint, mirroring how runServer wires it up.
func newSettingsTestEnv(t *testing.T) (http.HandlerFunc, *db.DB) {
	t.Helper()

	d, err := db.Open(filepath.Join(t.TempDir(), "eazy-gateway.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	return SettingsHandler(d, NewSessionStore()), d
}

// getSettings performs the public GET /api/settings request and decodes the body.
func getSettings(t *testing.T, h http.HandlerFunc) map[string]interface{} {
	t.Helper()

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want status 200, got %d (%s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

// The version is what a user quotes in a bug report and what the settings page
// displays, so the public settings endpoint has to report the running build.
func TestSettingsReportsBuildVersion(t *testing.T) {
	h, _ := newSettingsTestEnv(t)

	original := version.Version
	version.Version = "v9.9.9-test"
	t.Cleanup(func() { version.Version = original })

	if got := getSettings(t, h)["version"]; got != "v9.9.9-test" {
		t.Fatalf("want the running build version %q, got %#v", "v9.9.9-test", got)
	}
}

// The version must be read per request, not captured when the handler is built:
// main injects it at link time but tests (and future callers) change it later.
func TestSettingsReadsVersionPerRequest(t *testing.T) {
	h, _ := newSettingsTestEnv(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })

	version.Version = "first"
	first := getSettings(t, h)["version"]
	version.Version = "second"
	second := getSettings(t, h)["version"]

	if first != "first" || second != "second" {
		t.Fatalf("want first/second, got %#v/%#v", first, second)
	}
}

func TestSettingsKeepsExistingFields(t *testing.T) {
	h, d := newSettingsTestEnv(t)

	if err := d.SetSetting("appName", "My Gateway"); err != nil {
		t.Fatalf("set appName: %v", err)
	}
	if err := d.SetSetting("trafficTrendHours", "6"); err != nil {
		t.Fatalf("set trafficTrendHours: %v", err)
	}

	body := getSettings(t, h)
	if body["appName"] != "My Gateway" {
		t.Errorf("want appName %q, got %#v", "My Gateway", body["appName"])
	}
	if hours, ok := body["trafficTrendHours"].(float64); !ok || hours != 6 {
		t.Errorf("want trafficTrendHours 6, got %#v", body["trafficTrendHours"])
	}
}

// GET is public by design; PUT must stay behind a session.
func TestSettingsPutStillRequiresSession(t *testing.T) {
	h, d := newSettingsTestEnv(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"appName":"hijacked"}`))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want status 401 for an unauthenticated update, got %d", rec.Code)
	}
	if name, _ := d.GetSetting("appName"); name != "" {
		t.Fatalf("unauthenticated update must not persist, appName is now %q", name)
	}
}

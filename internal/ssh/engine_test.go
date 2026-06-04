package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// testConfig returns a config that points at a port where nothing is
// listening, so Connect/Start reliably fail fast without needing a real SSH
// server. Port 1 is privileged and unused; the dial is refused quickly.
func testConfig(id string) *Config {
	return &Config{
		ID:      id,
		Name:    "test-" + id,
		Type:    "local",
		SSHHost: "127.0.0.1",
		SSHPort: 1,
		SSHUser: "tester",
	}
}

// testKeyPEM is generated at runtime so ParsePrivateKey always succeeds and
// Connect/Start failures surface at dial time, the path under test.
var testKeyPEM = generateTestKey()

func generateTestKey() []byte {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "test@tun-console")
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(block)
}

func TestStatusUnknownTunnel(t *testing.T) {
	e := NewTunnelEngine()
	if got := e.Status("nope"); got != StatusDisconnected {
		t.Fatalf("Status(unknown) = %q, want %q", got, StatusDisconnected)
	}
}

func TestStartUnregistered(t *testing.T) {
	e := NewTunnelEngine()
	if err := e.Start("missing"); err == nil {
		t.Fatal("Start(unregistered) = nil error, want error")
	}
}

func TestStopUnknown(t *testing.T) {
	e := NewTunnelEngine()
	if err := e.Stop("missing"); err == nil {
		t.Fatal("Stop(unknown) = nil error, want error")
	}
}

func TestRegisterValidation(t *testing.T) {
	e := NewTunnelEngine()
	if err := e.Register(nil, nil); err == nil {
		t.Fatal("Register(nil) = nil error, want error")
	}
	if err := e.Register(&Config{}, nil); err == nil {
		t.Fatal("Register(empty ID) = nil error, want error")
	}
}

// TestStartConnectFailure verifies that an initial connect failure yields
// StatusError, returns an error, and starts no supervisor goroutine to leak.
func TestStartConnectFailure(t *testing.T) {
	e := NewTunnelEngine()
	cfg := testConfig("t1")
	if err := e.Register(cfg, testKeyPEM); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := e.Start("t1"); err == nil {
		t.Fatal("Start with refused dial = nil error, want error")
	}
	if got := e.Status("t1"); got != StatusError {
		t.Fatalf("Status after failed Start = %q, want %q", got, StatusError)
	}
	// A tunnel left in error may be retried; it should still fail, not panic
	// or report "already running".
	if err := e.Start("t1"); err == nil {
		t.Fatal("retry Start = nil error, want error")
	}
}

func TestConnectInvalidKey(t *testing.T) {
	cfg := testConfig("bad")
	if _, err := Connect(cfg, []byte("not a pem key")); err == nil {
		t.Fatal("Connect with bad key = nil error, want error")
	}
}

func TestConnectNilConfig(t *testing.T) {
	if _, err := Connect(nil, testKeyPEM); err == nil {
		t.Fatal("Connect(nil config) = nil error, want error")
	}
}

func TestDeregisterClearsKey(t *testing.T) {
	e := NewTunnelEngine()
	cfg := testConfig("t2")
	if err := e.Register(cfg, testKeyPEM); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if e.getKey("t2") == nil {
		t.Fatal("expected cached key after Register")
	}
	e.Deregister("t2")
	if e.getKey("t2") != nil {
		t.Fatal("expected nil cached key after Deregister")
	}
	if err := e.Start("t2"); err == nil {
		t.Fatal("Start after Deregister = nil error, want error")
	}
}

// TestConcurrentAccess hammers the engine from many goroutines to surface
// data races (run with -race) and to confirm no operation panics.
func TestConcurrentAccess(t *testing.T) {
	e := NewTunnelEngine()
	const workers = 25
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			id := "tunnel"
			cfg := testConfig(id)
			_ = e.Register(cfg, testKeyPEM)
			_ = e.Start(id)
			_ = e.Status(id)
			_ = e.Stop(id)
			e.Deregister(id)
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent access test timed out (possible deadlock)")
	}

	e.Shutdown()
}

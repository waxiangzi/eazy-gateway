package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Tunnel status values returned by TunnelEngine.Status.
const (
	StatusConnected    = "connected"
	StatusConnecting   = "connecting"
	StatusDisconnected = "disconnected"
	StatusError        = "error"
)

// Health-check and reconnect tuning.
const (
	healthCheckInterval = 5 * time.Second
	initialBackoff      = 1 * time.Second
	maxBackoff          = 60 * time.Second
	keepaliveRequest    = "keepalive@openssh.com"
)

// TunnelEngine manages the lifecycle of multiple SSH tunnel connections.
//
// It is safe for concurrent use: Start, Stop, Status, Register, and
// Deregister may all be called from multiple goroutines. Three categories
// of shared state are tracked, each guarded so accesses never race:
//
//   - configs:  registered *Config per tunnel ID, reused on reconnect.
//   - keyCache: in-memory decrypted private keys per tunnel ID (never
//     written to disk), reused on reconnect.
//   - tunnels:  the set of active (started) tunnels and their live state.
//
// configs and keyCache persist across Stop so a tunnel can be restarted or
// transparently reconnected; call Deregister to purge them (and zero the
// cached key) when a tunnel is permanently removed.
type TunnelEngine struct {
	mu      sync.RWMutex
	configs map[string]*Config
	tunnels map[string]*tunnel

	keyMu    sync.RWMutex
	keyCache map[string][]byte
}

// NewTunnelEngine returns an initialized, empty TunnelEngine.
func NewTunnelEngine() *TunnelEngine {
	return &TunnelEngine{
		configs:  make(map[string]*Config),
		tunnels:  make(map[string]*tunnel),
		keyCache: make(map[string][]byte),
	}
}

// tunnel holds the live state for a single supervised SSH connection.
//
// config and engine are set once at creation and never mutated, so they are
// read without locking. All other fields are guarded by mu. The supervise
// goroutine is the only writer of client/status after Start returns, except
// that Stop closes the client after the goroutine has exited.
type tunnel struct {
	id     string
	config *Config
	engine *TunnelEngine

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.RWMutex
	client *ssh.Client
	status string
}

// Register stores a tunnel's configuration and its decrypted PEM private key
// for later use by Start and by automatic reconnects. The key is copied into
// an internal cache (never persisted to disk). Calling Register again for the
// same ID replaces the stored config and key.
func (e *TunnelEngine) Register(config *Config, keyPEM []byte) error {
	if config == nil {
		return fmt.Errorf("ssh engine: register nil config")
	}
	if config.ID == "" {
		return fmt.Errorf("ssh engine: register config with empty ID")
	}

	e.mu.Lock()
	e.configs[config.ID] = config
	e.mu.Unlock()

	e.cacheKey(config.ID, keyPEM)
	return nil
}

// Deregister permanently removes a tunnel's config and zeroes/clears its
// cached key. It does not stop an active tunnel; call Stop first. This is the
// correct way to ensure a decrypted key no longer resides in memory.
func (e *TunnelEngine) Deregister(tunnelID string) {
	e.mu.Lock()
	delete(e.configs, tunnelID)
	e.mu.Unlock()

	e.clearKey(tunnelID)
}

// Start brings a registered tunnel online. It performs a synchronous initial
// SSH connection; on success it stores the client, reports StatusConnected,
// and launches the health-check/reconnect supervisor goroutine. On initial
// failure it reports StatusError and returns the error (no goroutine is
// leaked, since none is started until the connection succeeds).
//
// The tunnel must have been registered via Register first. Calling Start on a
// tunnel that is already running returns an error; a tunnel left in
// StatusError from a previous failed Start may be retried.
func (e *TunnelEngine) Start(tunnelID string) error {
	e.mu.Lock()
	cfg, ok := e.configs[tunnelID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("ssh engine: tunnel %q not registered", tunnelID)
	}
	if existing, exists := e.tunnels[tunnelID]; exists {
		// A tunnel only lacks a live supervisor goroutine when its initial
		// connect failed (StatusError). Any other state means it is actively
		// connected or reconnecting, so refuse to start a duplicate.
		if existing.getStatus() != StatusError {
			e.mu.Unlock()
			return fmt.Errorf("ssh engine: tunnel %q already running", tunnelID)
		}
		// Replace the stale errored entry and start fresh.
		delete(e.tunnels, tunnelID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &tunnel{
		id:     tunnelID,
		config: cfg,
		engine: e,
		cancel: cancel,
		status: StatusConnecting,
	}
	e.tunnels[tunnelID] = t
	e.mu.Unlock()

	// Connect outside the engine lock: dialing can block up to the connect
	// timeout and must not stall other Start/Stop/Status calls.
	keyPEM := e.getKey(tunnelID)
	client, err := Connect(cfg, keyPEM)
	if err != nil {
		t.setStatus(StatusError)
		cancel() // nothing waits on it yet, but keep the context tidy.
		fmt.Printf("[ssh] tunnel %q initial connect failed: %v\n", tunnelID, err)
		return fmt.Errorf("ssh engine: start tunnel %q: %w", tunnelID, err)
	}

	t.mu.Lock()
	t.client = client
	t.status = StatusConnected
	t.mu.Unlock()

	t.wg.Add(1)
	go t.supervise(ctx)

	return nil
}

// Stop closes a tunnel's SSH connection, cancels its supervisor goroutine,
// and removes it from the active set. It blocks until the supervisor has
// fully exited, guaranteeing no goroutine leak. The tunnel's config and
// cached key are retained so it can be started again; use Deregister to purge
// them. Stopping an unknown tunnel is a no-op that returns an error.
func (e *TunnelEngine) Stop(tunnelID string) error {
	e.mu.Lock()
	t, ok := e.tunnels[tunnelID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("ssh engine: tunnel %q not active", tunnelID)
	}
	delete(e.tunnels, tunnelID)
	e.mu.Unlock()

	fmt.Printf("[ssh] stopping tunnel %q\n", tunnelID)

	// Signal the supervisor to stop, then wait for it to exit before closing
	// the client so there is no concurrent access to t.client.
	t.cancel()
	t.wg.Wait()
	t.closeClient()
	t.setStatus(StatusDisconnected)

	fmt.Printf("[ssh] stopped tunnel %q\n", tunnelID)
	return nil
}

// Status reports the current state of a tunnel: StatusConnected,
// StatusConnecting, StatusDisconnected, or StatusError. An unknown or stopped
// tunnel is reported as StatusDisconnected.
func (e *TunnelEngine) Status(tunnelID string) string {
	e.mu.RLock()
	t, ok := e.tunnels[tunnelID]
	e.mu.RUnlock()
	if !ok {
		return StatusDisconnected
	}
	return t.getStatus()
}

// Shutdown stops every active tunnel and waits for all supervisor goroutines
// to exit. It is a convenience for clean process shutdown and leaves no
// goroutines running.
func (e *TunnelEngine) Shutdown() {
	e.mu.RLock()
	ids := make([]string, 0, len(e.tunnels))
	for id := range e.tunnels {
		ids = append(ids, id)
	}
	e.mu.RUnlock()

	for _, id := range ids {
		_ = e.Stop(id)
	}
}

// supervise runs the health-check loop for a connected tunnel. Every
// healthCheckInterval it sends an SSH keepalive; if the keepalive fails the
// connection is considered dead and it enters the reconnect loop. The loop
// exits only when ctx is cancelled (via Stop/Shutdown), guaranteeing the
// goroutine does not leak.
func (t *tunnel) supervise(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.keepalive() {
				continue
			}
			fmt.Printf("[ssh] tunnel %q health check failed; reconnecting\n", t.id)
			if !t.reconnect(ctx) {
				// reconnect returns false only when ctx was cancelled.
				return
			}
			fmt.Printf("[ssh] tunnel %q reconnected\n", t.id)
		}
	}
}

// keepalive sends an SSH global keepalive request with wantReply=true and
// reports whether the connection is still healthy. A nil client or any
// transport error is treated as unhealthy.
func (t *tunnel) keepalive() bool {
	t.mu.RLock()
	client := t.client
	t.mu.RUnlock()
	if client == nil {
		return false
	}
	_, _, err := client.SendRequest(keepaliveRequest, true, nil)
	return err == nil
}

// reconnect tears down the dead connection and repeatedly attempts to
// re-establish it using the tunnel's stored config and cached key, backing
// off exponentially (1s, 2s, 4s, 8s, ... capped at maxBackoff). It returns
// true once reconnected, or false if ctx is cancelled first. The cached key
// is read under the engine's key lock on every attempt so it stays current.
func (t *tunnel) reconnect(ctx context.Context) bool {
	t.setStatus(StatusDisconnected)
	t.closeClient()

	backoff := initialBackoff
	for {
		// Bail out promptly if we were asked to stop.
		select {
		case <-ctx.Done():
			return false
		default:
		}

		t.setStatus(StatusConnecting)
		keyPEM := t.engine.getKey(t.id)
		client, err := Connect(t.config, keyPEM)
		if err == nil {
			t.mu.Lock()
			t.client = client
			t.status = StatusConnected
			t.mu.Unlock()
			return true
		}

		t.setStatus(StatusDisconnected)
		fmt.Printf("[ssh] tunnel %q reconnect attempt failed: %v (retry in %s)\n", t.id, err, backoff)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// closeClient closes and clears the tunnel's SSH client, if any. Safe to call
// when no client is set.
func (t *tunnel) closeClient() {
	t.mu.Lock()
	client := t.client
	t.client = nil
	t.mu.Unlock()
	if client != nil {
		_ = client.Close()
	}
}

// getStatus returns the tunnel's current status string under read lock.
func (t *tunnel) getStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// setStatus updates the tunnel's status string under write lock.
func (t *tunnel) setStatus(status string) {
	t.mu.Lock()
	t.status = status
	t.mu.Unlock()
}

// cacheKey stores a private-key copy in the in-memory cache under the key
// lock. The caller's slice is copied so later mutation cannot corrupt it.
func (e *TunnelEngine) cacheKey(id string, keyPEM []byte) {
	cp := make([]byte, len(keyPEM))
	copy(cp, keyPEM)

	e.keyMu.Lock()
	e.keyCache[id] = cp
	e.keyMu.Unlock()
}

// getKey returns a copy of the cached decrypted key for a tunnel, or nil if
// none is cached. A copy is returned so the caller can use it without holding
// the key lock, immune to a concurrent clearKey zeroing the cached array.
func (e *TunnelEngine) getKey(id string) []byte {
	e.keyMu.RLock()
	defer e.keyMu.RUnlock()
	src, ok := e.keyCache[id]
	if !ok {
		return nil
	}
	cp := make([]byte, len(src))
	copy(cp, src)
	return cp
}

// clearKey zeroes and removes a tunnel's cached key so the decrypted material
// no longer lingers in memory.
func (e *TunnelEngine) clearKey(id string) {
	e.keyMu.Lock()
	if key, ok := e.keyCache[id]; ok {
		for i := range key {
			key[i] = 0
		}
		delete(e.keyCache, id)
	}
	e.keyMu.Unlock()
}

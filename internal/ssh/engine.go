package ssh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
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
//
// Forwarding state (listeners and active connections) is guarded by fwdMu,
// a lock separate from mu so that accepting/relaying goroutines never contend
// with the health-check/status path. fwdWg tracks every forwarding goroutine
// (accept loops and per-connection relays) so Stop can wait for a clean
// shutdown with no goroutine leak.
type tunnel struct {
	id     string
	config *Config
	engine *TunnelEngine

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.RWMutex
	client *ssh.Client
	status string

	fwdMu     sync.Mutex
	listeners []net.Listener
	conns     map[net.Conn]struct{}
	fwdWg     sync.WaitGroup

	bytesIn  atomic.Uint64
	bytesOut atomic.Uint64
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
		conns:  make(map[net.Conn]struct{}),
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
		slog.Error("initial connect failed", "tunnel", tunnelID, "error", err)
		return fmt.Errorf("ssh engine: start tunnel %q: %w", tunnelID, err)
	}

	t.mu.Lock()
	t.client = client
	t.status = StatusConnected
	t.mu.Unlock()

	t.startForwarding()

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

	slog.Info("stopping tunnel", "tunnel", tunnelID)

	// Signal the supervisor to stop, then wait for it to exit before closing
	// the client so there is no concurrent access to t.client. Forwarding is
	// torn down first so its listeners and relays stop before the transport.
	t.cancel()
	t.wg.Wait()
	t.stopForwarding()
	t.closeClient()
	t.setStatus(StatusDisconnected)

	slog.Info("stopped tunnel", "tunnel", tunnelID)
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
			slog.Warn("health check failed; reconnecting", "tunnel", t.id)
			if !t.reconnect(ctx) {
				// reconnect returns false only when ctx was cancelled.
				return
			}
			slog.Info("reconnected", "tunnel", t.id)
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
	t.stopForwarding()
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
			t.startForwarding()
			return true
		}

		t.setStatus(StatusDisconnected)
		slog.Warn("reconnect attempt failed", "tunnel", t.id, "error", err, "retryIn", backoff)

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

// getClient returns the tunnel's current SSH client under read lock, or nil if
// none is connected. Forwarding goroutines call this on every dial so they
// always use the latest client after a reconnect.
func (t *tunnel) getClient() *ssh.Client {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.client
}

// startForwarding launches the forwarder appropriate to the tunnel's type. It
// runs after every successful (re)connect and re-arms the active-connection
// set that the preceding stopForwarding nilled out. Unknown types are a no-op.
func (t *tunnel) startForwarding() {
	t.fwdMu.Lock()
	t.conns = make(map[net.Conn]struct{})
	t.fwdMu.Unlock()

	switch t.config.Type {
	case "local":
		t.startLocalForward()
	case "remote":
		t.startRemoteForward()
	case "dynamic":
		t.startDynamicForward()
	}
}

// stopForwarding tears down all forwarding for the tunnel. It nils the
// connection set so any goroutine still racing to register a freshly accepted
// connection aborts instead of leaking, closes every listener (unblocking
// accept loops) and every active connection (unblocking in-flight relays),
// then waits for all forwarding goroutines to exit. Safe to call when nothing
// is forwarding.
func (t *tunnel) stopForwarding() {
	t.fwdMu.Lock()
	listeners := t.listeners
	t.listeners = nil
	conns := t.conns
	t.conns = nil
	t.fwdMu.Unlock()

	for _, l := range listeners {
		_ = l.Close()
	}
	for c := range conns {
		_ = c.Close()
	}

	t.fwdWg.Wait()
}

// addListener records a listener so stopForwarding can close it.
func (t *tunnel) addListener(l net.Listener) {
	t.fwdMu.Lock()
	t.listeners = append(t.listeners, l)
	t.fwdMu.Unlock()
}

// trackConn registers an active forwarded connection so stopForwarding can
// close it to unblock a stalled relay. It returns false once stopForwarding has
// begun (conns nilled); the caller must then close the connection itself rather
// than relay it, which closes the race window where a connection accepted
// during shutdown would otherwise relay forever and hang fwdWg.Wait.
func (t *tunnel) trackConn(c net.Conn) bool {
	t.fwdMu.Lock()
	defer t.fwdMu.Unlock()
	if t.conns == nil {
		return false
	}
	t.conns[c] = struct{}{}
	return true
}

// untrackConn removes a connection from the active set once its relay is done.
func (t *tunnel) untrackConn(c net.Conn) {
	t.fwdMu.Lock()
	if t.conns != nil {
		delete(t.conns, c)
	}
	t.fwdMu.Unlock()
}

// acceptLoop accepts connections on l until it is closed, handing each off to
// handle in its own tracked goroutine. It owns one fwdWg count (added by the
// caller before launching it) and adds another per accepted connection, so
// stopForwarding's fwdWg.Wait covers both the loop and every relay. handle is
// responsible for closing the connection it is given.
func (t *tunnel) acceptLoop(l net.Listener, handle func(net.Conn)) {
	defer t.fwdWg.Done()
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		if !t.trackConn(conn) {
			_ = conn.Close()
			return
		}
		t.fwdWg.Add(1)
		go func(c net.Conn) {
			defer t.fwdWg.Done()
			defer t.untrackConn(c)
			handle(c)
		}(conn)
	}
}

// pipe relays bytes in both directions between a and b until either side hits
// EOF or an error, then closes both ends so the opposite copy also unblocks.
// It is the per-connection workhorse shared by all three forwarders.
// Traffic is counted atomically: bytesIn = remote->local, bytesOut = local->remote.
func (t *tunnel) pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(a, b)
		t.bytesIn.Add(uint64(n))
		_ = a.Close()
		_ = b.Close()
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(b, a)
		t.bytesOut.Add(uint64(n))
		_ = a.Close()
		_ = b.Close()
	}()
	wg.Wait()
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

// Traffic returns the current runtime byte counters for this tunnel.
func (t *tunnel) Traffic() (uint64, uint64) {
	return t.bytesIn.Load(), t.bytesOut.Load()
}

// Traffic returns the live byte counters for an active tunnel, or zeroes if
// the tunnel is not currently running.
func (e *TunnelEngine) Traffic(tunnelID string) (uint64, uint64) {
	e.mu.RLock()
	t, ok := e.tunnels[tunnelID]
	e.mu.RUnlock()
	if !ok {
		return 0, 0
	}
	return t.Traffic()
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

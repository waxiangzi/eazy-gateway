package ssh

import (
	"log/slog"
	"net"
)

// startLocalForward implements SSH local port forwarding (-L). It listens on
// the tunnel's LocalAddr and, for each accepted connection, dials RemoteAddr
// through the SSH connection so traffic to the local port is delivered to the
// remote address as seen from the SSH server.
//
// The listener and every relayed connection are tracked by the tunnel so Stop
// (and reconnect) can close them and wait for the goroutines to exit. A
// missing LocalAddr or RemoteAddr is logged and skipped rather than treated as
// fatal, since the transport connection itself is still useful.
func (t *tunnel) startLocalForward() {
	local := t.config.LocalAddr
	remote := t.config.RemoteAddr
	if local == "" || remote == "" {
		slog.Warn("local forward skipped", "tunnel", t.id, "localAddr", local, "remoteAddr", remote)
		return
	}

	l, err := net.Listen("tcp", local)
	if err != nil {
		slog.Error("local forward listen failed", "tunnel", t.id, "addr", local, "error", err)
		return
	}
	t.addListener(l)
	slog.Info("forwarding -L", "tunnel", t.id, "local", local, "remote", remote)

	t.fwdWg.Add(1)
	go t.acceptLoop(l, func(c net.Conn) {
		t.handleLocalConn(c, remote)
	})
}

// handleLocalConn dials remote through the current SSH client and relays the
// accepted local connection to it. It always closes the local connection; the
// remote side is closed by pipe.
func (t *tunnel) handleLocalConn(local net.Conn, remote string) {
	client := t.getClient()
	if client == nil {
		_ = local.Close()
		return
	}
	remoteConn, err := client.Dial("tcp", remote)
	if err != nil {
		slog.Error("local forward dial failed", "tunnel", t.id, "addr", remote, "error", err)
		_ = local.Close()
		return
	}
	t.pipe(local, remoteConn)
}

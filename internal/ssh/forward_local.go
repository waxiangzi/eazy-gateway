package ssh

import (
	"fmt"
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
		fmt.Printf("[ssh] tunnel %q local forward skipped: localAddr=%q remoteAddr=%q\n", t.id, local, remote)
		return
	}

	l, err := net.Listen("tcp", local)
	if err != nil {
		fmt.Printf("[ssh] tunnel %q local forward listen %s failed: %v\n", t.id, local, err)
		return
	}
	t.addListener(l)
	fmt.Printf("[ssh] tunnel %q forwarding -L %s -> %s\n", t.id, local, remote)

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
		fmt.Printf("[ssh] tunnel %q local forward dial %s failed: %v\n", t.id, remote, err)
		_ = local.Close()
		return
	}
	t.pipe(local, remoteConn)
}

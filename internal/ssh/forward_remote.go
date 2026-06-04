package ssh

import (
	"fmt"
	"net"
)

// startRemoteForward implements SSH remote port forwarding (-R). It asks the
// SSH server to open a listener on RemoteAddr via client.Listen; each
// connection the server accepts there is dialed to LocalAddr locally and the
// two are relayed.
//
// Some SSH servers disable remote forwarding (AllowTcpForwarding no /
// GatewayPorts), in which case client.Listen returns an error. That is handled
// gracefully: it is logged and the forward is skipped, leaving the transport
// connection intact rather than failing the whole tunnel.
func (t *tunnel) startRemoteForward() {
	remote := t.config.RemoteAddr
	local := t.config.LocalAddr
	if remote == "" || local == "" {
		fmt.Printf("[ssh] tunnel %q remote forward skipped: remoteAddr=%q localAddr=%q\n", t.id, remote, local)
		return
	}

	client := t.getClient()
	if client == nil {
		return
	}

	l, err := client.Listen("tcp", remote)
	if err != nil {
		fmt.Printf("[ssh] tunnel %q remote forward listen %s rejected by server: %v\n", t.id, remote, err)
		return
	}
	t.addListener(l)
	fmt.Printf("[ssh] tunnel %q forwarding -R %s -> %s\n", t.id, remote, local)

	t.fwdWg.Add(1)
	go t.acceptLoop(l, func(c net.Conn) {
		t.handleRemoteConn(c, local)
	})
}

// handleRemoteConn dials local with the host's network stack and relays the
// server-accepted connection to it. It always closes the remote connection;
// the local side is closed by pipe.
func (t *tunnel) handleRemoteConn(remote net.Conn, local string) {
	localConn, err := net.Dial("tcp", local)
	if err != nil {
		fmt.Printf("[ssh] tunnel %q remote forward dial %s failed: %v\n", t.id, local, err)
		_ = remote.Close()
		return
	}
	t.pipe(remote, localConn)
}

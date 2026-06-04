package ssh

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
)

// SOCKS5 protocol constants (RFC 1928). Only the subset needed for a no-auth
// CONNECT proxy is defined.
const (
	socks5Version    = 0x05
	socks5NoAuth     = 0x00
	socks5NoAccept   = 0xFF
	socks5CmdConnect = 0x01

	socks5AtypIPv4   = 0x01
	socks5AtypDomain = 0x03
	socks5AtypIPv6   = 0x04

	socks5RepSuccess        = 0x00
	socks5RepGeneralFailure = 0x01
	socks5RepCmdNotSupp     = 0x07
	socks5RepAtypNotSupp    = 0x08
)

// startDynamicForward implements SSH dynamic forwarding (-D): a minimal SOCKS5
// proxy listening on DynamicAddr. Each accepted client speaks SOCKS5; its
// CONNECT target is dialed through the SSH connection and relayed, so the SSH
// server acts as the exit for arbitrary outbound TCP.
func (t *tunnel) startDynamicForward() {
	dynamic := t.config.DynamicAddr
	if dynamic == "" {
		fmt.Printf("[ssh] tunnel %q dynamic forward skipped: dynamicAddr empty\n", t.id)
		return
	}

	l, err := net.Listen("tcp", dynamic)
	if err != nil {
		fmt.Printf("[ssh] tunnel %q dynamic forward listen %s failed: %v\n", t.id, dynamic, err)
		return
	}
	t.addListener(l)
	fmt.Printf("[ssh] tunnel %q forwarding -D %s (SOCKS5)\n", t.id, dynamic)

	t.fwdWg.Add(1)
	go t.acceptLoop(l, t.handleSocksConn)
}

// handleSocksConn runs the SOCKS5 handshake and CONNECT exchange on a client
// connection, then relays it to the dialed target. It always closes the client
// connection on any failure; on success the target side is closed by pipe.
func (t *tunnel) handleSocksConn(client net.Conn) {
	target, err := t.socksNegotiate(client)
	if err != nil {
		fmt.Printf("[ssh] tunnel %q SOCKS5 negotiate failed: %v\n", t.id, err)
		_ = client.Close()
		return
	}
	t.pipe(client, target)
}

// socksNegotiate performs method selection and reads the CONNECT request,
// dials the requested target through the SSH client, writes the SOCKS5 reply,
// and returns the dialed target connection ready for relaying. On any error it
// writes a SOCKS5 failure reply when the protocol stage allows and returns the
// error; the caller closes the client connection.
func (t *tunnel) socksNegotiate(client net.Conn) (net.Conn, error) {
	// Method selection: VER, NMETHODS, METHODS...
	header := make([]byte, 2)
	if _, err := io.ReadFull(client, header); err != nil {
		return nil, fmt.Errorf("read greeting: %w", err)
	}
	if header[0] != socks5Version {
		return nil, fmt.Errorf("unsupported version 0x%02x", header[0])
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(client, methods); err != nil {
		return nil, fmt.Errorf("read methods: %w", err)
	}
	if !slices.Contains(methods, socks5NoAuth) {
		_, _ = client.Write([]byte{socks5Version, socks5NoAccept})
		return nil, fmt.Errorf("no acceptable auth method")
	}
	if _, err := client.Write([]byte{socks5Version, socks5NoAuth}); err != nil {
		return nil, fmt.Errorf("write method selection: %w", err)
	}

	// Request: VER, CMD, RSV, ATYP, DST.ADDR, DST.PORT.
	reqHead := make([]byte, 4)
	if _, err := io.ReadFull(client, reqHead); err != nil {
		return nil, fmt.Errorf("read request: %w", err)
	}
	if reqHead[0] != socks5Version {
		return nil, fmt.Errorf("unsupported request version 0x%02x", reqHead[0])
	}
	if reqHead[1] != socks5CmdConnect {
		t.writeSocksReply(client, socks5RepCmdNotSupp)
		return nil, fmt.Errorf("unsupported command 0x%02x", reqHead[1])
	}

	host, err := readSocksAddr(client, reqHead[3])
	if err != nil {
		t.writeSocksReply(client, socks5RepAtypNotSupp)
		return nil, err
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(client, portBytes); err != nil {
		return nil, fmt.Errorf("read port: %w", err)
	}
	target := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(portBytes))))

	sshClient := t.getClient()
	if sshClient == nil {
		t.writeSocksReply(client, socks5RepGeneralFailure)
		return nil, fmt.Errorf("no SSH connection")
	}
	targetConn, err := sshClient.Dial("tcp", target)
	if err != nil {
		t.writeSocksReply(client, socks5RepGeneralFailure)
		return nil, fmt.Errorf("dial %s: %w", target, err)
	}

	if err := t.writeSocksReply(client, socks5RepSuccess); err != nil {
		_ = targetConn.Close()
		return nil, fmt.Errorf("write success reply: %w", err)
	}
	return targetConn, nil
}

// writeSocksReply writes a SOCKS5 reply with the given status and a zeroed
// IPv4 bound address, which is sufficient for a CONNECT proxy.
func (t *tunnel) writeSocksReply(client net.Conn, status byte) error {
	_, err := client.Write([]byte{socks5Version, status, 0x00, socks5AtypIPv4, 0, 0, 0, 0, 0, 0})
	return err
}

// readSocksAddr reads a SOCKS5 destination address of the given ATYP and
// returns it as a host string suitable for net.JoinHostPort.
func readSocksAddr(client net.Conn, atyp byte) (string, error) {
	switch atyp {
	case socks5AtypIPv4:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(client, buf); err != nil {
			return "", fmt.Errorf("read ipv4: %w", err)
		}
		return net.IP(buf).String(), nil
	case socks5AtypIPv6:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(client, buf); err != nil {
			return "", fmt.Errorf("read ipv6: %w", err)
		}
		return net.IP(buf).String(), nil
	case socks5AtypDomain:
		lenByte := make([]byte, 1)
		if _, err := io.ReadFull(client, lenByte); err != nil {
			return "", fmt.Errorf("read domain length: %w", err)
		}
		domain := make([]byte, int(lenByte[0]))
		if _, err := io.ReadFull(client, domain); err != nil {
			return "", fmt.Errorf("read domain: %w", err)
		}
		return string(domain), nil
	default:
		return "", fmt.Errorf("unsupported address type 0x%02x", atyp)
	}
}

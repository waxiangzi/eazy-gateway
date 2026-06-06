package ssh

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const socks5DialTimeout = 15 * time.Second

// DialSOCKS5 connects to a SOCKS5 proxy at proxyAddr and requests a CONNECT to targetAddr.
// It returns the established TCP connection ready for relaying.
func DialSOCKS5(proxyAddr, targetAddr string) (net.Conn, error) {
	proxy, err := net.DialTimeout("tcp", proxyAddr, socks5DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("dial socks5 proxy %s: %w", proxyAddr, err)
	}

	if err := socks5Handshake(proxy); err != nil {
		_ = proxy.Close()
		return nil, err
	}

	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		_ = proxy.Close()
		return nil, fmt.Errorf("split target addr %s: %w", targetAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		_ = proxy.Close()
		return nil, fmt.Errorf("parse target port %s: %w", portStr, err)
	}

	if err := socks5Request(proxy, host, port); err != nil {
		_ = proxy.Close()
		return nil, err
	}

	return proxy, nil
}

func socks5Handshake(conn net.Conn) error {
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("socks5 write greeting: %w", err)
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks5 read greeting: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("socks5 unsupported version 0x%02x", resp[0])
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("socks5 auth method 0x%02x not accepted", resp[1])
	}
	return nil
}

func socks5Request(conn net.Conn, host string, port int) error {
	ip := net.ParseIP(host)
	var req []byte
	if ip4 := ip.To4(); ip4 != nil {
		req = append([]byte{0x05, 0x01, 0x00, 0x01}, ip4...)
	} else if ip6 := ip.To16(); ip6 != nil {
		req = append([]byte{0x05, 0x01, 0x00, 0x04}, ip6...)
	} else {
		req = append([]byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}, host...)
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	req = append(req, portBytes...)

	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5 write connect request: %w", err)
	}

	resp := make([]byte, 4)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks5 read connect response: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("socks5 response version 0x%02x", resp[0])
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("socks5 connect failed status 0x%02x", resp[1])
	}

	switch resp[3] {
	case 0x01:
		if _, err := io.ReadFull(conn, make([]byte, 4+2)); err != nil {
			return fmt.Errorf("socks5 read ipv4 bound: %w", err)
		}
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return fmt.Errorf("socks5 read domain len: %w", err)
		}
		if _, err := io.ReadFull(conn, make([]byte, int(lenBuf[0])+2)); err != nil {
			return fmt.Errorf("socks5 read domain bound: %w", err)
		}
	case 0x04:
		if _, err := io.ReadFull(conn, make([]byte, 16+2)); err != nil {
			return fmt.Errorf("socks5 read ipv6 bound: %w", err)
		}
	default:
		return fmt.Errorf("socks5 unsupported bound type 0x%02x", resp[3])
	}
	return nil
}

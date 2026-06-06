package ssh

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// startHTTPToSocks5Forward implements an HTTP proxy that forwards traffic
// through an upstream SOCKS5 proxy. It supports both CONNECT tunnelling and
// direct HTTP relay. Dynamic proxy rules (domain-pattern → SOCKS5) are honoured.
func (t *tunnel) startHTTPToSocks5Forward() {
	local := t.config.LocalAddr
	if local == "" {
		slog.Warn("http-to-socks5 forward skipped", "tunnel", t.id, "localAddr", local)
		return
	}

	l, err := net.Listen("tcp", local)
	if err != nil {
		slog.Error("http-to-socks5 listen failed", "tunnel", t.id, "addr", local, "error", err)
		return
	}
	t.addListener(l)
	slog.Info("forwarding HTTP→SOCKS5", "tunnel", t.id, "addr", local)

	t.fwdWg.Add(1)
	go t.acceptLoop(l, t.handleHTTPProxyConn)
}

// handleHTTPProxyConn handles a single client connection. It detects whether
// the first request is CONNECT (tunnel) or a plain HTTP request and dispatches
// accordingly. For direct HTTP it processes a single request-response pair and
// then closes the connection.
func (t *tunnel) handleHTTPProxyConn(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		slog.Debug("http-to-socks5 read request failed", "tunnel", t.id, "error", err)
		return
	}

	if req.Method == http.MethodConnect {
		t.handleHTTPConnect(conn, req)
		return
	}
	t.handleHTTPDirect(conn, req)
}

// handleHTTPConnect processes a CONNECT request: resolves the upstream SOCKS5
// proxy, dials the target through it, replies 200 to the client, then pipes.
func (t *tunnel) handleHTTPConnect(client net.Conn, req *http.Request) {
	proxyAddr, target, err := t.resolveProxy(req.Host)
	if err != nil {
		slog.Error("http-to-socks5 resolve proxy failed", "tunnel", t.id, "target", req.Host, "error", err)
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
		return
	}

	targetConn, err := DialSOCKS5(proxyAddr, target)
	if err != nil {
		slog.Error("http-to-socks5 dial target failed", "tunnel", t.id, "target", target, "error", err)
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
		return
	}

	if _, err := client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		_ = targetConn.Close()
		return
	}

	t.pipe(client, targetConn)
}

// handleHTTPDirect relays a single non-CONNECT HTTP request through the
// upstream SOCKS5 proxy and writes the response back to the client.
func (t *tunnel) handleHTTPDirect(client net.Conn, req *http.Request) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	proxyAddr, target, err := t.resolveProxy(host)
	if err != nil {
		slog.Error("http-to-socks5 resolve proxy failed", "tunnel", t.id, "target", host, "error", err)
		writeHTTPError(client, http.StatusBadGateway, "no upstream proxy")
		return
	}

	targetConn, err := DialSOCKS5(proxyAddr, target)
	if err != nil {
		slog.Error("http-to-socks5 dial target failed", "tunnel", t.id, "target", target, "error", err)
		writeHTTPError(client, http.StatusBadGateway, "upstream connect failed")
		return
	}
	defer func() { _ = targetConn.Close() }()

	if err := req.Write(targetConn); err != nil {
		slog.Error("http-to-socks5 write request failed", "tunnel", t.id, "error", err)
		writeHTTPError(client, http.StatusBadGateway, "upstream write failed")
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(targetConn), req)
	if err != nil {
		slog.Error("http-to-socks5 read response failed", "tunnel", t.id, "error", err)
		writeHTTPError(client, http.StatusBadGateway, "upstream read failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if err := resp.Write(client); err != nil {
		slog.Debug("http-to-socks5 write response failed", "tunnel", t.id, "error", err)
	}
}

// resolveProxy selects a SOCKS5 upstream for the given target. If proxy rules
// are configured it returns the first matching rule, otherwise it falls back to
// the tunnel's default Socks5Host/Socks5Port.
func (t *tunnel) resolveProxy(target string) (proxyAddr string, resolvedTarget string, err error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host = target
		port = "80"
	}
	resolvedTarget = net.JoinHostPort(host, port)

	for _, rule := range t.config.ProxyRules {
		if matchDomain(rule.DomainPattern, host) {
			return net.JoinHostPort(rule.Socks5Host, strconv.Itoa(rule.Socks5Port)), resolvedTarget, nil
		}
	}

	if t.config.Socks5Host != "" && t.config.Socks5Port > 0 {
		return net.JoinHostPort(t.config.Socks5Host, strconv.Itoa(t.config.Socks5Port)), resolvedTarget, nil
	}

	return "", "", fmt.Errorf("no upstream proxy for %s", target)
}

// matchDomain returns true if domain matches pattern. Patterns support a leading
// "*." wildcard that matches any subdomain (but not the domain itself).
func matchDomain(pattern, domain string) bool {
	if pattern == domain {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(domain, pattern[1:])
	}
	return false
}

func writeHTTPError(conn net.Conn, code int, msg string) {
	body := []byte(msg + "\n")
	fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", code, http.StatusText(code), len(body), body)
}

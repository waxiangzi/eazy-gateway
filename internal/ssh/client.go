// Package ssh implements the SSH tunnel engine core: establishing SSH
// client connections, managing their lifecycle, health checking, and
// auto-reconnection with exponential backoff.
//
// Forwarding logic (-L / -R / -D) is intentionally NOT implemented here;
// it is handled by later tasks. This package only owns the SSH transport
// connection and its supervision.
package ssh

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// connectTimeout bounds how long a single dial+handshake attempt may take.
const connectTimeout = 15 * time.Second

type ProxyRule struct {
	DomainPattern string
	Socks5Host    string
	Socks5Port    int
}

// Config describes a single SSH tunnel's connection parameters.
//
// It is defined locally in this package because the canonical
// db.TunnelConfig type (internal/db) is introduced by a later task. The
// field set mirrors the planned db.TunnelConfig so that switching to the
// shared type later is mechanical. The engine stores a *Config per tunnel
// so it can be reused verbatim during reconnects.
type Config struct {
	ID string
	Name string
	Type string

	SSHHost string
	SSHPort int
	SSHUser string

	LocalAddr   string
	RemoteAddr  string
	DynamicAddr string

	Socks5Host string
	Socks5Port int
	ProxyRules []ProxyRule
}

// addr returns the "host:port" dial target for the SSH server.
func (c *Config) addr() string {
	return net.JoinHostPort(c.SSHHost, strconv.Itoa(c.SSHPort))
}

// Connect establishes an SSH client connection to the server described by
// config, authenticating with the supplied PEM-encoded private key bytes.
//
// The host key is deliberately NOT verified: per the project guardrail we
// use ssh.InsecureIgnoreHostKey() and emit a warning. keyPEM must be the
// already-decrypted private key in PEM form; this function never touches
// disk and never logs key material.
//
// The returned *ssh.Client is owned by the caller, which is responsible for
// closing it. Any failure (key parse, dial, handshake, auth) is returned as
// an error and never panics.
func Connect(config *Config, keyPEM []byte) (*ssh.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("ssh connect: nil config")
	}
	if config.SSHHost == "" {
		return nil, fmt.Errorf("ssh connect: empty SSHHost")
	}
	if config.SSHPort <= 0 {
		return nil, fmt.Errorf("ssh connect: invalid SSHPort %d", config.SSHPort)
	}
	if config.SSHUser == "" {
		return nil, fmt.Errorf("ssh connect: empty SSHUser")
	}

	// Parse the PEM private key into a signer for public-key auth.
	signer, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("ssh connect: parse private key: %w", err)
	}

	// SECURITY WARNING: host key verification is disabled per project
	// guardrail. This accepts any server host key, which is vulnerable to
	// man-in-the-middle attacks. See InsecureIgnoreHostKey usage below.
	slog.Warn("host key verification disabled (InsecureIgnoreHostKey)", "tunnel", config.ID, "addr", config.addr())

	clientConfig := &ssh.ClientConfig{
		User: config.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         connectTimeout,
	}

	addr := config.addr()
	slog.Info("dialing tunnel", "tunnel", config.ID, "addr", addr, "user", config.SSHUser)

	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh connect: dial %s: %w", addr, err)
	}

	slog.Info("connected tunnel", "tunnel", config.ID, "addr", addr)
	return client, nil
}

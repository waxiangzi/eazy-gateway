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
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// connectTimeout bounds how long a single dial+handshake attempt may take.
const connectTimeout = 15 * time.Second

// Config describes a single SSH tunnel's connection parameters.
//
// It is defined locally in this package because the canonical
// db.TunnelConfig type (internal/db) is introduced by a later task. The
// field set mirrors the planned db.TunnelConfig so that switching to the
// shared type later is mechanical. The engine stores a *Config per tunnel
// so it can be reused verbatim during reconnects.
type Config struct {
	// ID uniquely identifies the tunnel and is used as the engine map key.
	ID string
	// Name is a human-friendly label for logging.
	Name string
	// Type is one of "local", "remote", or "dynamic". Reserved for the
	// forwarding tasks; unused by the connection layer.
	Type string

	// SSHHost is the SSH server hostname or IP to dial.
	SSHHost string
	// SSHPort is the SSH server TCP port.
	SSHPort int
	// SSHUser is the SSH login user.
	SSHUser string

	// Forwarding addresses, reserved for later tasks (-L/-R/-D).
	LocalAddr   string
	RemoteAddr  string
	DynamicAddr string
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
	fmt.Printf("[ssh] WARNING: host key verification disabled (InsecureIgnoreHostKey) for tunnel %q -> %s\n",
		config.ID, config.addr())

	clientConfig := &ssh.ClientConfig{
		User: config.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         connectTimeout,
	}

	addr := config.addr()
	fmt.Printf("[ssh] dialing tunnel %q (%s) as user %q\n", config.ID, addr, config.SSHUser)

	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh connect: dial %s: %w", addr, err)
	}

	fmt.Printf("[ssh] connected tunnel %q (%s)\n", config.ID, addr)
	return client, nil
}

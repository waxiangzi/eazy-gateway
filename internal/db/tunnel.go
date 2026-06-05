package db

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

// TunnelType represents the kind of SSH tunnel.
type TunnelType string

const (
	TunnelTypeLocal   TunnelType = "local"
	TunnelTypeRemote  TunnelType = "remote"
	TunnelTypeDynamic TunnelType = "dynamic"
)

// TunnelConfig holds the configuration for a single SSH tunnel.
type TunnelConfig struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         TunnelType `json:"type"`
	HostID       string     `json:"hostId"`
	ListenPort   int        `json:"listenPort"`
	TargetHost   string     `json:"targetHost,omitempty"`
	TargetPort   int        `json:"targetPort"`
	BindExternal bool       `json:"bindExternal"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// ListenAddr returns the bind address for this tunnel based on BindExternal.
func (t *TunnelConfig) ListenAddr() string {
	host := "127.0.0.1"
	if t.BindExternal {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, strconv.Itoa(t.ListenPort))
}

// TargetAddr returns the target address for local/remote forwarding.
func (t *TunnelConfig) TargetAddr() string {
	if t.TargetHost == "" {
		return ""
	}
	return net.JoinHostPort(t.TargetHost, strconv.Itoa(t.TargetPort))
}

// generateID returns a random hex string (32 hex chars = 128 bits).
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateTunnel stores a new tunnel configuration and returns its assigned ID.
func (db *DB) CreateTunnel(t *TunnelConfig) (string, error) {
	id, err := generateID()
	if err != nil {
		return "", err
	}
	t.ID = id
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTunnels)
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshal tunnel: %w", err)
		}
		return b.Put([]byte(id), data)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetTunnel retrieves a tunnel configuration by ID. Returns nil if not found.
func (db *DB) GetTunnel(id string) (*TunnelConfig, error) {
	var t *TunnelConfig
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTunnels)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		var tc TunnelConfig
		if err := json.Unmarshal(data, &tc); err != nil {
			return fmt.Errorf("unmarshal tunnel: %w", err)
		}
		t = &tc
		return nil
	})
	return t, err
}

// UpdateTunnel updates an existing tunnel configuration.
func (db *DB) UpdateTunnel(t *TunnelConfig) error {
	t.UpdatedAt = time.Now().UTC()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTunnels)
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshal tunnel: %w", err)
		}
		return b.Put([]byte(t.ID), data)
	})
}

// DeleteTunnel removes a tunnel configuration by ID.
func (db *DB) DeleteTunnel(id string) error {
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketTunnels).Delete([]byte(id))
	})
}

// ListTunnels returns all tunnel configurations.
func (db *DB) ListTunnels() ([]*TunnelConfig, error) {
	var tunnels []*TunnelConfig
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTunnels)
		return b.ForEach(func(k, v []byte) error {
			var tc TunnelConfig
			if err := json.Unmarshal(v, &tc); err != nil {
				return fmt.Errorf("unmarshal tunnel %q: %w", string(k), err)
			}
			tunnels = append(tunnels, &tc)
			return nil
		})
	})
	return tunnels, err
}

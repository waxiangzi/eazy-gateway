package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
	"github.com/eazy-gateway/eazy-gateway/internal/ssh"
)

// TunnelHandler holds shared dependencies for tunnel CRUD handlers.
// engine is optional; when set, Delete also stops any running tunnel and the
// Start/Stop endpoints become operational.
type TunnelHandler struct {
	db     *db.DB
	engine *ssh.TunnelEngine
}

// NewTunnelHandler creates a TunnelHandler with the given dependencies.
func NewTunnelHandler(d *db.DB, engine *ssh.TunnelEngine) *TunnelHandler {
	return &TunnelHandler{db: d, engine: engine}
}

type proxyRuleRequest struct {
	DomainPattern string `json:"domainPattern"`
	Socks5Host    string `json:"socks5Host"`
	Socks5Port    int    `json:"socks5Port"`
}

// tunnelRequest is the JSON body accepted by Create and Update.
type tunnelRequest struct {
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	HostID       string             `json:"hostId"`
	ListenPort   int                `json:"listenPort"`
	TargetHost   string             `json:"targetHost,omitempty"`
	TargetPort   int                `json:"targetPort"`
	BindExternal bool               `json:"bindExternal"`
	Socks5Host   string             `json:"socks5Host,omitempty"`
	Socks5Port   int                `json:"socks5Port,omitempty"`
	ProxyRules   []proxyRuleRequest `json:"proxyRules,omitempty"`
}

// validate checks that all required fields are present and valid.
func (h *TunnelHandler) validate(req *tunnelRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(req.Name) > 128 || hasControlChar(req.Name) {
		return fmt.Errorf("name must not exceed 128 characters or contain control characters")
	}
	switch req.Type {
	case "local", "remote", "dynamic", "httpToSocks5":
	default:
		return fmt.Errorf("type must be one of: local, remote, dynamic, httpToSocks5")
	}
	if req.ListenPort <= 0 || req.ListenPort > 65535 {
		return fmt.Errorf("listenPort must be between 1 and 65535")
	}
	if req.Type == "httpToSocks5" {
		if len(req.ProxyRules) == 0 {
			if req.Socks5Host == "" {
				return fmt.Errorf("socks5Host is required")
			}
			if req.Socks5Port <= 0 || req.Socks5Port > 65535 {
				return fmt.Errorf("socks5Port must be between 1 and 65535")
			}
		}
		for _, rule := range req.ProxyRules {
			if rule.DomainPattern == "" {
				return fmt.Errorf("proxyRules domainPattern is required")
			}
			if rule.Socks5Host == "" {
				return fmt.Errorf("proxyRules socks5Host is required")
			}
			if rule.Socks5Port <= 0 || rule.Socks5Port > 65535 {
				return fmt.Errorf("proxyRules socks5Port must be between 1 and 65535")
			}
		}
		return nil
	}
	if req.HostID == "" {
		return fmt.Errorf("hostId is required")
	}
	host, err := h.db.GetHost(req.HostID)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}
	if host == nil {
		return fmt.Errorf("host %q not found", req.HostID)
	}
	if req.Type != "dynamic" {
		if req.TargetHost == "" {
			return fmt.Errorf("targetHost is required")
		}
		if len(req.TargetHost) > 253 || hasControlChar(req.TargetHost) {
			return fmt.Errorf("targetHost must not exceed 253 characters or contain control characters")
		}
		if req.TargetPort <= 0 || req.TargetPort > 65535 {
			return fmt.Errorf("targetPort must be between 1 and 65535")
		}
	}
	return nil
}

// toDBTunnel converts a tunnelRequest + ID into a db.TunnelConfig.
func toDBTunnel(id string, req *tunnelRequest) *db.TunnelConfig {
	proxyRules := make([]db.ProxyRule, 0, len(req.ProxyRules))
	for _, r := range req.ProxyRules {
		proxyRules = append(proxyRules, db.ProxyRule{
			DomainPattern: r.DomainPattern,
			Socks5Host:    r.Socks5Host,
			Socks5Port:    r.Socks5Port,
		})
	}
	return &db.TunnelConfig{
		ID:           id,
		Name:         req.Name,
		Type:         db.TunnelType(req.Type),
		HostID:       req.HostID,
		ListenPort:   req.ListenPort,
		TargetHost:   req.TargetHost,
		TargetPort:   req.TargetPort,
		BindExternal: req.BindExternal,
		Socks5Host:   req.Socks5Host,
		Socks5Port:   req.Socks5Port,
		ProxyRules:   proxyRules,
	}
}

// toSSHConfig converts a db.TunnelConfig and its host into an ssh.Config.
func toSSHConfig(tc *db.TunnelConfig, host *db.Host) *ssh.Config {
	cfg := &ssh.Config{
		ID:         tc.ID,
		Name:       tc.Name,
		Type:       string(tc.Type),
		Socks5Host: tc.Socks5Host,
		Socks5Port: tc.Socks5Port,
		ProxyRules: make([]ssh.ProxyRule, 0, len(tc.ProxyRules)),
	}
	for _, r := range tc.ProxyRules {
		cfg.ProxyRules = append(cfg.ProxyRules, ssh.ProxyRule{
			DomainPattern: r.DomainPattern,
			Socks5Host:    r.Socks5Host,
			Socks5Port:    r.Socks5Port,
		})
	}
	if host != nil {
		cfg.SSHHost = host.Host
		cfg.SSHPort = host.Port
		cfg.SSHUser = host.User
	}
	switch tc.Type {
	case db.TunnelTypeLocal:
		cfg.LocalAddr = tc.ListenAddr()
		cfg.RemoteAddr = tc.TargetAddr()
	case db.TunnelTypeRemote:
		cfg.RemoteAddr = tc.ListenAddr()
		cfg.LocalAddr = tc.TargetAddr()
	case db.TunnelTypeDynamic:
		cfg.DynamicAddr = tc.ListenAddr()
	case db.TunnelTypeHTTPToSocks5:
		cfg.LocalAddr = tc.ListenAddr()
	}
	return cfg
}

// List handles GET /api/tunnels — returns all tunnel configs with runtime status.
func (h *TunnelHandler) List(w http.ResponseWriter, r *http.Request) {
	tunnels, err := h.db.ListTunnels()
	if err != nil {
		log.Printf("ERROR: list tunnels: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if tunnels == nil {
		tunnels = []*db.TunnelConfig{}
	}

	hosts, err := h.db.ListHosts()
	if err != nil {
		log.Printf("ERROR: list hosts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	hostMap := make(map[string]*db.Host, len(hosts))
	for _, host := range hosts {
		hostMap[host.ID] = host
	}

	type traffic struct {
		BytesIn  uint64 `json:"bytesIn"`
		BytesOut uint64 `json:"bytesOut"`
	}

	type item struct {
		db.TunnelConfig
		Status  string  `json:"status"`
		Traffic traffic `json:"traffic"`
	}

	items := make([]item, 0, len(tunnels))
	for _, t := range tunnels {
		s := ssh.StatusDisconnected
		if h.engine != nil {
			s = h.engine.Status(t.ID)
		}
		rtIn, rtOut := uint64(0), uint64(0)
		if h.engine != nil {
			rtIn, rtOut = h.engine.Traffic(t.ID)
		}
		stored, _ := h.db.GetTraffic(t.ID)
		if stored == nil {
			stored = &db.TrafficStats{}
		}
		items = append(items, item{
			TunnelConfig: *t,
			Status:       s,
			Traffic: traffic{
				BytesIn:  stored.TotalBytesIn + rtIn,
				BytesOut: stored.TotalBytesOut + rtOut,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"hosts": hostMap,
	})
}

// Get handles GET /api/tunnels/{id} — returns a single tunnel config with runtime status.
func (h *TunnelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	status := ssh.StatusDisconnected
	if h.engine != nil {
		status = h.engine.Status(id)
	}

	rtIn, rtOut := uint64(0), uint64(0)
	if h.engine != nil {
		rtIn, rtOut = h.engine.Traffic(id)
	}
	stored, _ := h.db.GetTraffic(id)
	if stored == nil {
		stored = &db.TrafficStats{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tunnel": tc,
		"status": status,
		"traffic": map[string]uint64{
			"bytesIn":  stored.TotalBytesIn + rtIn,
			"bytesOut": stored.TotalBytesOut + rtOut,
		},
	})
}

// Status handles GET /api/tunnels/{id}/status — returns tunnel config with runtime status.
func (h *TunnelHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	status := ssh.StatusDisconnected
	if h.engine != nil {
		status = h.engine.Status(id)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tunnel": tc,
		"status": status,
	})
}

func (h *TunnelHandler) RestoreEnabled() {
	if h.engine == nil {
		return
	}

	tunnels, err := h.db.ListTunnels()
	if err != nil {
		log.Printf("ERROR: list tunnels for restore: %v", err)
		return
	}

	hosts, err := h.db.ListHosts()
	if err != nil {
		log.Printf("ERROR: list hosts for restore: %v", err)
		return
	}
	hostMap := make(map[string]*db.Host, len(hosts))
	for _, host := range hosts {
		hostMap[host.ID] = host
	}

	for _, tc := range tunnels {
		if !tc.Enabled {
			continue
		}
			s := h.engine.Status(tc.ID)
		if s == ssh.StatusConnected || s == ssh.StatusConnecting {
			log.Printf("INFO: tunnel %q (%s) already %s, skipping restore", tc.ID, tc.Name, s)
			continue
		}

		var host *db.Host
		var keyPEM []byte
		if tc.Type != db.TunnelTypeHTTPToSocks5 {
			host = hostMap[tc.HostID]
			if host == nil {
				log.Printf("WARN: tunnel %q enabled but host %q not found, skipping", tc.ID, tc.HostID)
				continue
			}
			key, err := h.db.GetKey(host.KeyID)
			if err != nil || key == nil {
				log.Printf("WARN: tunnel %q enabled but key for host %q not found: %v", tc.ID, host.ID, err)
				continue
			}
			keyPEM, err = os.ReadFile(key.PrivateKeyPath)
			if err != nil {
				log.Printf("WARN: tunnel %q enabled but key file unreadable: %v", tc.ID, err)
				continue
			}
		}

		if err := h.engine.Register(toSSHConfig(tc, host), keyPEM); err != nil {
			log.Printf("WARN: tunnel %q register failed on restore: %v", tc.ID, err)
			continue
		}
		if err := h.engine.Start(tc.ID); err != nil {
			log.Printf("WARN: tunnel %q start failed on restore: %v", tc.ID, err)
			continue
		}
		log.Printf("INFO: restored tunnel %q (%s)", tc.ID, tc.Name)
	}
}

// Create handles POST /api/tunnels — creates a new tunnel config.
func (h *TunnelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req tunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := h.db.CreateTunnel(toDBTunnel("", &req))
	if err != nil {
		log.Printf("ERROR: create tunnel: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// Update handles PUT /api/tunnels/{id} — updates an existing tunnel config.
func (h *TunnelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	// Tunnel must exist
	existing, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	var req tunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tc := toDBTunnel(id, &req)
	tc.CreatedAt = existing.CreatedAt // preserve original creation time
	tc.Enabled = existing.Enabled       // preserve enabled state

	if err := h.db.UpdateTunnel(tc); err != nil {
		log.Printf("ERROR: update tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete handles DELETE /api/tunnels/{id} — deletes a tunnel config.
// If the tunnel is currently running and an engine is available, it stops
// the tunnel before deleting the config.
func (h *TunnelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	// Verify tunnel exists
	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	// Stop the tunnel if it is currently running and persist runtime traffic.
	if h.engine != nil {
		if status := h.engine.Status(id); status != ssh.StatusDisconnected {
			log.Printf("stopping running tunnel %q before delete", id)
			rtIn, rtOut := h.engine.Traffic(id)
			if err := h.engine.Stop(id); err != nil {
				log.Printf("WARN: stop tunnel %q before delete: %v", id, err)
				// Continue with deletion even if stop fails
			}
			h.engine.Deregister(id)
			stored, _ := h.db.GetTraffic(id)
			if stored == nil {
				stored = &db.TrafficStats{}
			}
			stored.TotalBytesIn += rtIn
			stored.TotalBytesOut += rtOut
			_ = h.db.UpdateTraffic(id, stored)
		}
	}

	if err := h.db.DeleteTunnel(id); err != nil {
		log.Printf("ERROR: delete tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	_ = h.db.DeleteTraffic(id)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Start handles POST /api/tunnels/{id}/start. It loads the tunnel config,
// resolves its host, reads the host's key, registers both with the engine,
// and brings the tunnel online.
func (h *TunnelHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if h.engine == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "engine not available"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	var host *db.Host
	var keyPEM []byte
	if tc.Type != db.TunnelTypeHTTPToSocks5 {
		var err error
		host, err = h.db.GetHost(tc.HostID)
		if err != nil {
			log.Printf("ERROR: get host %q for tunnel %q: %v", tc.HostID, id, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if host == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "connection host not found"})
			return
		}

		key, err := h.db.GetKey(host.KeyID)
		if err != nil {
			log.Printf("ERROR: get key %q for host %q: %v", host.KeyID, host.ID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if key == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "host key not found"})
			return
		}

		keyPEM, err = os.ReadFile(key.PrivateKeyPath)
		if err != nil {
			log.Printf("ERROR: read private key file %q for tunnel %q: %v", key.PrivateKeyPath, id, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read key"})
			return
		}
	}

	if err := h.engine.Register(toSSHConfig(tc, host), keyPEM); err != nil {
		log.Printf("ERROR: register tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := h.engine.Start(id); err != nil {
		if strings.Contains(err.Error(), "already running") {
			log.Printf("INFO: tunnel %q start requested but already running", id)
			writeJSON(w, http.StatusOK, map[string]string{"status": "already running"})
			return
		}
		log.Printf("ERROR: start tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	tc.Enabled = true
	if err := h.db.UpdateTunnel(tc); err != nil {
		log.Printf("WARN: set tunnel %q enabled=true: %v", id, err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// Stop handles POST /api/tunnels/{id}/stop. It validates the tunnel exists and
// delegates to the engine, which tears down forwarding and the SSH connection.
func (h *TunnelHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if h.engine == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "engine not available"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	// Persist runtime traffic before stopping.
	rtIn, rtOut := uint64(0), uint64(0)
	if h.engine != nil {
		rtIn, rtOut = h.engine.Traffic(id)
	}

	if err := h.engine.Stop(id); err != nil {
		if strings.Contains(err.Error(), "not active") {
			log.Printf("INFO: stop tunnel %q: already inactive", id)
		} else {
			log.Printf("ERROR: stop tunnel %q: %v", id, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
	}

	stored, _ := h.db.GetTraffic(id)
	if stored == nil {
		stored = &db.TrafficStats{}
	}
	stored.TotalBytesIn += rtIn
	stored.TotalBytesOut += rtOut
	_ = h.db.UpdateTraffic(id, stored)

	tc.Enabled = false
	if err := h.db.UpdateTunnel(tc); err != nil {
		log.Printf("WARN: set tunnel %q enabled=false: %v", id, err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// TrafficTrend handles GET /api/tunnels/{id}/traffic/trend.
// It returns delta traffic points for the requested look-back window.
func (h *TunnelHandler) TrafficTrend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	hours := 1
	if hStr := r.URL.Query().Get("hours"); hStr != "" {
		if v, err := strconv.Atoi(hStr); err == nil && v > 0 && v <= 48 {
			hours = v
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	samples, err := h.db.ListTrafficSamples(id, since)
	if err != nil {
		log.Printf("ERROR: list traffic samples %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	rtIn, rtOut := uint64(0), uint64(0)
	if h.engine != nil {
		rtIn, rtOut = h.engine.Traffic(id)
	}
	stored, _ := h.db.GetTraffic(id)
	if stored == nil {
		stored = &db.TrafficStats{}
	}
	totalIn := stored.TotalBytesIn + rtIn
	totalOut := stored.TotalBytesOut + rtOut
	now := time.Now().Unix()

	type point struct {
		Timestamp int64 `json:"timestamp"`
		BytesIn   int64 `json:"bytesIn"`
		BytesOut  int64 `json:"bytesOut"`
	}

	var points []point
	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		curr := samples[i]
		if curr.Timestamp <= prev.Timestamp {
			continue
		}
		var dIn, dOut int64
		if curr.BytesIn > prev.BytesIn {
			dIn = int64(curr.BytesIn - prev.BytesIn)
		}
		if curr.BytesOut > prev.BytesOut {
			dOut = int64(curr.BytesOut - prev.BytesOut)
		}
		points = append(points, point{
			Timestamp: curr.Timestamp,
			BytesIn:   dIn,
			BytesOut:  dOut,
		})
	}

	if len(samples) > 0 {
		last := samples[len(samples)-1]
		if now > last.Timestamp {
			var dIn, dOut int64
			if totalIn > last.BytesIn {
				dIn = int64(totalIn - last.BytesIn)
			}
			if totalOut > last.BytesOut {
				dOut = int64(totalOut - last.BytesOut)
			}
			points = append(points, point{
				Timestamp: now,
				BytesIn:   dIn,
				BytesOut:  dOut,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"points": points,
	})
}

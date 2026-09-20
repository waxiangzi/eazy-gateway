package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/eazy-gateway/eazy-gateway/internal/api"
	"github.com/eazy-gateway/eazy-gateway/internal/crypto"
	"github.com/eazy-gateway/eazy-gateway/internal/db"
	"github.com/eazy-gateway/eazy-gateway/internal/ssh"
)

func main() {
	serve := flag.Bool("serve", false, "Run in background")
	install := flag.Bool("install", false, "Install as systemd service")
	port := flag.Int("port", getEnvInt("PORT", 8022), "HTTP server port")
	dataDir := flag.String("data", getEnv("DATA_DIRECTORY", "./data"), "Data directory for persistent storage")
	secureCookies := flag.Bool("secure-cookies", false, "Set Secure flag on session cookies (enable when behind TLS-terminating proxy)")
	flag.Parse()

	if *install {
		installDataDir := *dataDir
		if installDataDir == "./data" {
			home, err := os.UserHomeDir()
			if err == nil {
				installDataDir = filepath.Join(home, ".eazy-gateway", "data")
			}
		}
		if err := installSystemd(*port, installDataDir); err != nil {
			fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully installed and started eazy-gateway as a user systemd service.")
		fmt.Printf("Data directory: %s\n", installDataDir)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		if *serve {
			daemonize(*port, *dataDir)
		}
		runServer(*port, *dataDir, *secureCookies)
	} else {
		runCommand(args[0], args[1:], *dataDir)
	}
}

func daemonize(port int, dataDir string) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "create data directory: %v\n", err)
		os.Exit(1)
	}

	pidfile := filepath.Join(dataDir, "eazy-gateway.pid")
	if data, err := os.ReadFile(pidfile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && processExists(pid) {
			fmt.Printf("Service already running (pid: %d)\n", pid)
			os.Exit(1)
		}
	}

	logFile := filepath.Join(dataDir, "eazy-gateway.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open log file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	cmd := exec.Command(os.Args[0],
		"--port", strconv.Itoa(port),
		"--data", dataDir,
	)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start daemon: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write pidfile: %v\n", err)
		_ = cmd.Process.Kill()
		os.Exit(1)
	}

	fmt.Printf("Service started, pid: %d\n", cmd.Process.Pid)
	os.Exit(0)
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func runServer(port int, dataDir string, secureCookies bool) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Error("create data directory", "error", err)
		os.Exit(1)
	}

	pidfile := filepath.Join(dataDir, "eazy-gateway.pid")
	if data, err := os.ReadFile(pidfile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && processExists(pid) && pid != os.Getpid() {
			logger.Error("another instance is already running", "pid", pid)
			os.Exit(1)
		}
	}
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
		logger.Error("write pidfile", "error", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(dataDir, "eazy-gateway.db")
	d, err := db.Open(dbPath)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}

	adminPassword, err := api.EnsureAdmin(d)
	if err != nil {
		logger.Error("ensure admin", "error", err)
		os.Exit(1)
	}

	// Resolve the password this process may use to decrypt stored private keys.
	// A fresh deployment just generated one; an existing deployment may still
	// have the initial generated password on file, which is verified against the
	// bcrypt hash before use. Otherwise the key store starts locked and an admin
	// login unlocks it.
	credential := ""
	if adminPassword != "" {
		credential = adminPassword
		logger.Info("admin password generated", "password", adminPassword)
		pwFile := filepath.Join(dataDir, "initial-password.txt")
		_ = os.WriteFile(pwFile, []byte(adminPassword+"\n"), 0o600)
	} else if pw, err := os.ReadFile(filepath.Join(dataDir, "initial-password.txt")); err == nil {
		candidate := strings.TrimRight(string(pw), "\n")
		if cfg, cfgErr := d.GetAdmin(); cfgErr == nil && cfg != nil && crypto.VerifyPassword(candidate, cfg.PasswordHash) {
			credential = candidate
		} else {
			logger.Info("initial-password.txt does not match the admin hash; key store starts locked")
		}
	}

	km := api.NewKeyManager(dataDir)
	api.SetCredential(credential)

	if err := km.EnsureDefaultKey(d); err != nil {
		logger.Error("ensure default key", "error", err)
		os.Exit(1)
	}

	// Upgrade any legacy plaintext private-key file to the encrypted format
	// when the store is unlocked.
	if err := km.UpgradeLegacyFiles(d); err != nil {
		logger.Error("upgrade legacy key files", "error", err)
		os.Exit(1)
	}

	api.SetSecureCookies(secureCookies)

	sessions := api.NewSessionStore()
	stopPruning := sessions.StartPruning(5 * time.Minute)
	defer stopPruning()

	keysHandler := api.NewKeysHandler(d, km)
	hostsHandler := api.NewHostsHandler(d, km)

	engine := ssh.NewTunnelEngine()
	tunnelsHandler := api.NewTunnelHandler(d, engine, km)

	// Restore tunnels that were enabled before last shutdown
	go tunnelsHandler.RestoreEnabled()

	// Background traffic sampler records cumulative counters every minute.
	go runTrafficSampler(d, engine, logger)

	mux := http.NewServeMux()
	var unlockOnce sync.Once
	mux.HandleFunc("/api/login", api.LoginHandler(d, sessions, func() {
		// Migrate any legacy plaintext key files under the password that just
		// unlocked the store, then bring the enabled tunnels online. Startup
		// already did both when it had a usable password, so this only does work
		// on the login that follows a locked start.
		unlockOnce.Do(func() {
			if err := km.UpgradeLegacyFiles(d); err != nil {
				logger.Error("upgrade legacy key files after login", "error", err)
			}
			if err := km.EnsureDefaultKey(d); err != nil {
				logger.Error("generate deferred default key after login", "error", err)
			}
			go tunnelsHandler.RestoreEnabled()
		})
	}))
	mux.HandleFunc("/api/logout", api.LogoutHandler(sessions))
	mux.HandleFunc("/api/me", api.MeHandler(sessions))
	mux.HandleFunc("/api/admin/change-password", api.ChangePasswordHandler(d, sessions, km, func() {
		// The original generated password is no longer valid.
		_ = os.Remove(filepath.Join(dataDir, "initial-password.txt"))
	}))
	mux.HandleFunc("/api/settings", api.SettingsHandler(d, sessions))
	mux.HandleFunc("GET /api/keys", keysHandler.HandleList)
	mux.HandleFunc("DELETE /api/keys/{id}", keysHandler.HandleDelete)
	mux.HandleFunc("GET /api/hosts", hostsHandler.List)
	mux.HandleFunc("POST /api/hosts", hostsHandler.Create)
	mux.HandleFunc("GET /api/hosts/{id}", hostsHandler.Get)
	mux.HandleFunc("PUT /api/hosts/{id}", hostsHandler.Update)
	mux.HandleFunc("DELETE /api/hosts/{id}", hostsHandler.Delete)
	mux.HandleFunc("POST /api/hosts/test", hostsHandler.Test)
	mux.HandleFunc("GET /api/tunnels", tunnelsHandler.List)
	mux.HandleFunc("GET /api/tunnels/{id}", tunnelsHandler.Get)
	mux.HandleFunc("POST /api/tunnels", tunnelsHandler.Create)
	mux.HandleFunc("PUT /api/tunnels/{id}", tunnelsHandler.Update)
	mux.HandleFunc("DELETE /api/tunnels/{id}", tunnelsHandler.Delete)
	mux.HandleFunc("POST /api/tunnels/{id}/start", tunnelsHandler.Start)
	mux.HandleFunc("POST /api/tunnels/{id}/stop", tunnelsHandler.Stop)
	mux.HandleFunc("GET /api/tunnels/{id}/traffic/trend", tunnelsHandler.TrafficTrend)

	authMW := api.NewAuthMiddleware(sessions)
	apiHandler := authMW.Wrap(mux)

	staticFS := getStaticFS()
	spaHandler := newSPAHandler(staticFS)

	rootMux := http.NewServeMux()
	rootMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	rootMux.Handle("/api/", apiHandler)
	rootMux.Handle("/", spaHandler)

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: withSecurityHeaders(rootMux),
	}

	// Unix socket for CLI communication (no auth required)
	socketPath := filepath.Join(dataDir, "cli.sock")
	_ = os.Remove(socketPath)
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		logger.Error("create unix socket", "error", err)
		os.Exit(1)
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		logger.Error("chmod unix socket", "error", err)
		os.Exit(1)
	}

	cliMux := http.NewServeMux()
	cliMux.HandleFunc("GET /api/tunnels", tunnelsHandler.List)
	cliMux.HandleFunc("GET /api/tunnels/{id}", tunnelsHandler.Get)
	cliMux.HandleFunc("GET /api/tunnels/{id}/status", tunnelsHandler.Status)
	cliMux.HandleFunc("POST /api/tunnels/{id}/start", tunnelsHandler.Start)
	cliMux.HandleFunc("POST /api/tunnels/{id}/stop", tunnelsHandler.Stop)
	cliMux.HandleFunc("POST /api/admin/reset-password", api.ResetPasswordHandler(d, sessions, km))
	// Also expose hosts API on CLI socket for potential future CLI commands
	cliMux.HandleFunc("GET /api/hosts", hostsHandler.List)
	cliMux.HandleFunc("GET /api/hosts/{id}", hostsHandler.Get)

	cliServer := &http.Server{
		Handler: cliMux,
	}

	var shutdownOnce sync.Once
	shutdown := func(reason string) {
		shutdownOnce.Do(func() {
			logger.Info("shutting down gracefully", "reason", reason)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				logger.Error("http server shutdown error", "error", err)
			}

			if err := cliServer.Shutdown(ctx); err != nil {
				logger.Error("unix socket server shutdown error", "error", err)
			}

			_ = os.Remove(socketPath)

			engine.Shutdown()
			if err := d.Close(); err != nil {
				logger.Error("database close error", "error", err)
			}

			_ = os.Remove(pidfile)

			logger.Info("shutdown complete")
		})
	}

	defer shutdown("normal exit")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal", "signal", sig.String())
		shutdown("signal")
	}()

	go func() {
		if err := cliServer.Serve(unixListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("unix socket server error", "error", err)
		}
	}()

	logger.Info("eazy-gateway starting", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		if isAddrInUse(err) {
			logger.Error("port already in use", "addr", server.Addr)
		} else {
			logger.Error("http server error", "error", err)
		}
		os.Exit(1)
	}
}

func runCommand(cmd string, args []string, dataDir string) {
	switch cmd {
	case "list":
		cmdList(dataDir)
	case "status":
		if len(args) < 1 {
			fmt.Println("Usage: eazy-gateway status <id>")
			os.Exit(1)
		}
		cmdStatus(args[0], dataDir)
	case "start":
		if len(args) < 1 {
			fmt.Println("Usage: eazy-gateway start <id>")
			os.Exit(1)
		}
		cmdStart(args[0], dataDir)
	case "stop":
		if len(args) < 1 {
			fmt.Println("Usage: eazy-gateway stop <id>")
			os.Exit(1)
		}
		cmdStop(args[0], dataDir)
	case "restart":
		if len(args) < 1 {
			fmt.Println("Usage: eazy-gateway restart <id>")
			os.Exit(1)
		}
		cmdRestart(args[0], dataDir)
	case "reset-password":
		cmdResetPassword(dataDir)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Usage: eazy-gateway [--install] [--port N] [--data dir] [command]")
		fmt.Println("Flags:")
		fmt.Println("  --install   Install as systemd service (requires sudo)")
		fmt.Println("  --port      HTTP server port (default 8022, env PORT)")
		fmt.Println("  --data      Data directory")
		fmt.Println("Commands: list, status, start, stop, restart, reset-password")
		os.Exit(1)
	}
}

func cliHTTPClient(dataDir string) *http.Client {
	socketPath := filepath.Join(dataDir, "cli.sock")
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
}

func checkService(dataDir string) {
	socketPath := filepath.Join(dataDir, "cli.sock")
	if _, err := os.Stat(socketPath); err != nil {
		fmt.Fprintln(os.Stderr, "Service not running")
		os.Exit(1)
	}
}

func cmdList(dataDir string) {
	checkService(dataDir)
	client := cliHTTPClient(dataDir)
	resp, err := client.Get("http://unix/api/tunnels")
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to service: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "service error: %s\n", string(body))
		os.Exit(1)
	}

	var result struct {
		Items []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Type         string `json:"type"`
			Status       string `json:"status"`
			HostID       string `json:"hostId"`
			ListenPort   int    `json:"listenPort"`
			BindExternal bool   `json:"bindExternal"`
		} `json:"items"`
		Hosts map[string]struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "parse response: %v\n", err)
		os.Exit(1)
	}

	if len(result.Items) == 0 {
		fmt.Println("No tunnels configured")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tSTATUS\tHOST\tLISTEN")
	for _, t := range result.Items {
		hostStr := "unknown"
		if h, ok := result.Hosts[t.HostID]; ok {
			hostStr = fmt.Sprintf("%s:%d", h.Host, h.Port)
		}
		listen := fmt.Sprintf("127.0.0.1:%d", t.ListenPort)
		if t.BindExternal {
			listen = fmt.Sprintf("0.0.0.0:%d", t.ListenPort)
		}
		id := t.ID
		if len(id) > 8 {
			id = id[:8]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", id, t.Name, t.Type, t.Status, hostStr, listen)
	}
	w.Flush()
}

func cmdStatus(id string, dataDir string) {
	checkService(dataDir)
	client := cliHTTPClient(dataDir)
	resp, err := client.Get("http://unix/api/tunnels/" + id + "/status")
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to service: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(os.Stderr, "Tunnel %q not found\n", id)
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "service error: %s\n", string(body))
		os.Exit(1)
	}

	var result struct {
		Tunnel struct {
			Name         string `json:"name"`
			Type         string `json:"type"`
			HostID       string `json:"hostId"`
			ListenPort   int    `json:"listenPort"`
			TargetHost   string `json:"targetHost"`
			TargetPort   int    `json:"targetPort"`
			BindExternal bool   `json:"bindExternal"`
		} `json:"tunnel"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "parse response: %v\n", err)
		os.Exit(1)
	}

	t := result.Tunnel
	fmt.Printf("Tunnel: %s\n", t.Name)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Type: %s\n", t.Type)

	listen := fmt.Sprintf("127.0.0.1:%d", t.ListenPort)
	if t.BindExternal {
		listen = fmt.Sprintf("0.0.0.0:%d", t.ListenPort)
	}
	fmt.Printf("Listen: %s\n", listen)

	if t.Type != "dynamic" && t.TargetHost != "" {
		fmt.Printf("Target: %s:%d\n", t.TargetHost, t.TargetPort)
	}

	// Resolve host info
	hostResp, err := client.Get("http://unix/api/hosts/" + t.HostID)
	if err == nil && hostResp.StatusCode == http.StatusOK {
		var host struct {
			Host string `json:"host"`
			Port int    `json:"port"`
			User string `json:"user"`
		}
		if err := json.NewDecoder(hostResp.Body).Decode(&host); err == nil {
			fmt.Printf("SSH: %s@%s:%d\n", host.User, host.Host, host.Port)
		}
		hostResp.Body.Close()
	}
}

func doTunnelAction(action, id, dataDir string) {
	client := cliHTTPClient(dataDir)
	url := fmt.Sprintf("http://unix/api/tunnels/%s/%s", id, action)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create request: %v\n", err)
		os.Exit(1)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to service: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(os.Stderr, "Tunnel %q not found\n", id)
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "service error: %s\n", string(body))
		os.Exit(1)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err == nil && result["status"] != "" {
		fmt.Printf("Tunnel %q %s\n", id, result["status"])
	} else {
		fmt.Printf("Tunnel %q %sed\n", id, action)
	}
}

func cmdStart(id string, dataDir string) {
	checkService(dataDir)
	doTunnelAction("start", id, dataDir)
}

func cmdStop(id string, dataDir string) {
	checkService(dataDir)
	doTunnelAction("stop", id, dataDir)
}

func cmdRestart(id string, dataDir string) {
	checkService(dataDir)
	client := cliHTTPClient(dataDir)
	stopReq, _ := http.NewRequest("POST", fmt.Sprintf("http://unix/api/tunnels/%s/stop", id), nil)
	stopResp, err := client.Do(stopReq)
	if err == nil && stopResp != nil {
		_, _ = io.Copy(io.Discard, stopResp.Body)
		stopResp.Body.Close()
	}
	doTunnelAction("start", id, dataDir)
}

func cmdResetPassword(dataDir string) {
	checkService(dataDir)
	client := cliHTTPClient(dataDir)

	resp, err := client.Post("http://unix/api/admin/reset-password", "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to service: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "service error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result struct {
		Password string `json:"password"`
		Warning  string `json:"warning"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "parse response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("========================================")
	fmt.Println("ADMIN PASSWORD RESET")
	fmt.Println("========================================")
	fmt.Printf("New password: %s\n", result.Password)
	fmt.Println("========================================")
	if result.Warning != "" {
		fmt.Printf("WARNING: %s\n", result.Warning)
	}
	fmt.Println("All existing sessions have been invalidated.")
	fmt.Println("Please log in with the new password.")
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if sysErr, ok := opErr.Err.(*syscall.Errno); ok {
			return *sysErr == syscall.EADDRINUSE
		}
	}
	return false
}

func newSPAHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			f, err := root.Open(path)
			if err != nil {
				r.URL.Path = "/"
			} else {
				f.Close()
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// unsafe-eval is required because vue-i18n compiles messages at runtime using new Function()
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func installSystemd(port int, dataDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	installDir := filepath.Join(home, ".eazy-gateway")
	binDir := filepath.Join(installDir, "bin")
	binPath := filepath.Join(binDir, "eazy-gateway")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data directory: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	if execPath != binPath {
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return fmt.Errorf("create bin directory: %w", err)
		}
		input, err := os.ReadFile(execPath)
		if err != nil {
			return fmt.Errorf("read current executable: %w", err)
		}
		if err := os.WriteFile(binPath, input, 0o755); err != nil {
			return fmt.Errorf("copy executable: %w", err)
		}
	}

	unit := fmt.Sprintf(`[Unit]
Description=Eazy-Gateway Tunnel Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --port %d --data %s
WorkingDirectory=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, shellQuote(binPath), port, shellQuote(absDataDir), shellQuote(absDataDir))

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}
	unitPath := filepath.Join(unitDir, "eazy-gateway.service")
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload: %w", err)
	}
	if err := exec.Command("systemctl", "--user", "enable", "eazy-gateway.service").Run(); err != nil {
		return fmt.Errorf("systemctl --user enable: %w", err)
	}
	if err := exec.Command("systemctl", "--user", "start", "eazy-gateway.service").Run(); err != nil {
		return fmt.Errorf("systemctl --user start: %w", err)
	}
	return nil
}

func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t\n\"'$&;|<>#*?[]{}()\\") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// runTrafficSampler periodically records cumulative traffic snapshots for every
// configured tunnel and prunes samples older than the configured retention window.
func runTrafficSampler(d *db.DB, engine *ssh.TunnelEngine, logger *slog.Logger) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		tunnels, err := d.ListTunnels()
		if err != nil {
			logger.Error("traffic sampler: list tunnels", "error", err)
			continue
		}

		now := time.Now().Unix()
		for _, t := range tunnels {
			rtIn, rtOut := uint64(0), uint64(0)
			if engine != nil {
				rtIn, rtOut = engine.Traffic(t.ID)
			}
			stored, _ := d.GetTraffic(t.ID)
			if stored == nil {
				stored = &db.TrafficStats{}
			}
			sample := &db.TrafficSample{
				Timestamp: now,
				BytesIn:   stored.TotalBytesIn + rtIn,
				BytesOut:  stored.TotalBytesOut + rtOut,
			}
			if err := d.RecordTrafficSample(t.ID, sample); err != nil {
				logger.Error("traffic sampler: record sample", "tunnel", t.ID, "error", err)
			}
		}

		retentionStr, _ := d.GetSetting("trafficTrendHours")
		hours, _ := strconv.Atoi(retentionStr)
		if hours <= 0 {
			hours = 1
		}
		cutoff := time.Now().Add(-time.Duration(hours+1) * time.Hour)
		if err := d.CleanupTrafficSamples(cutoff); err != nil {
			logger.Error("traffic sampler: cleanup", "error", err)
		}
	}
}

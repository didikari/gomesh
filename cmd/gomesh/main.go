package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gomesh/internal/agent/embeddedcfg"
	"gomesh/internal/server/auth"
	"gomesh/internal/server/hub"
	"gomesh/internal/server/store"
	"gomesh/internal/version"
	"gomesh/scripts"
	"gomesh/web"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server listening port")
	dbPath := flag.String("db", "data/gomesh.db", "Path to SQLite database file")
	showVersion := flag.Bool("version", false, "Print server version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("GoMesh Central Server %s\n", version.Get())
		return
	}

	// Structured text logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting GoMesh Central Server", "version", version.Get(), "port", *port, "db", *dbPath)

	st, err := store.New(*dbPath)
	if err != nil {
		slog.Error("failed to initialize sqlite store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	// Ensure default admin user
	if admin, created, err := st.EnsureDefaultAdmin(context.Background()); err != nil {
		slog.Error("failed initializing admin user", "error", err)
	} else if created {
		slog.Info("initialized default admin credentials", "username", admin.Username, "initial_password", "admin123")
	}

	// Ensure at least one active enrollment token
	tokens, err := st.ListEnrollmentTokens(context.Background())
	if err == nil && len(tokens) == 0 {
		tok, err := st.CreateEnrollmentToken(context.Background(), "Default Agent Enrollment Token", "system", 0)
		if err == nil {
			slog.Info("created initial agent enrollment token", "token", tok.Token)
		}
	}

	sm := auth.NewSessionManager(24 * time.Hour)
	h := hub.New(st)
	defer h.Stop()

	mux := buildServerMux(st, sm, h)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info(fmt.Sprintf("GoMesh Web Console ready at http://localhost:%d", *port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server listener error", "error", err)
		}
	}()

	// Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down GoMesh Central Server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server shutdown forced", "error", err)
	}

	slog.Info("GoMesh Central Server stopped cleanly")
}

func buildServerMux(st *store.Store, sm *auth.SessionManager, h *hub.Hub) *http.ServeMux {
	mux := http.NewServeMux()

	// Helper to serve embedded HTML asset
	serveHTML := func(w http.ResponseWriter, filename string) {
		data, err := web.Assets.ReadFile(filename)
		if err != nil {
			http.Error(w, "Asset not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		content := strings.ReplaceAll(string(data), "{{APP_VERSION}}", version.Get())
		w.Write([]byte(content))
	}

	// GET /login: Dedicated login page
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		if sess, ok := sm.GetSessionFromRequest(r); ok && sess != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		serveHTML(w, "login.html")
	})

	// GET /: Dedicated dashboard console (Protected by session guard)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if sess, ok := sm.GetSessionFromRequest(r); !ok || sess == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		serveHTML(w, "dashboard.html")
	})

	// GET /static/{path...}: Static CSS and JS assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(web.StaticFS))))

	// Serves install scripts (embedded with fallback to disk)
	mux.HandleFunc("GET /scripts/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := filepath.Base(r.PathValue("file"))
		if data, err := scripts.Files.ReadFile(file); err == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write(data)
			return
		}
		candidates := []string{
			filepath.Join("scripts", file),
			filepath.Join("..", "scripts", file),
		}
		for _, p := range candidates {
			if data, err := os.ReadFile(p); err == nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Write(data)
				return
			}
		}
		http.Error(w, "Script not found", http.StatusNotFound)
	})

	// Helper to normalize any HTTP/HTTPS/WS/WSS address to a WebSocket URL
	normalizeWS := func(raw string) string {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return ""
		}
		if !strings.Contains(raw, "://") {
			raw = "http://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return ""
		}
		proto := "ws"
		if u.Scheme == "https" || u.Scheme == "wss" {
			proto = "wss"
		}
		path := u.Path
		if path == "" || path == "/" {
			path = "/ws/agent"
		} else if !strings.HasSuffix(path, "/ws/agent") {
			path = strings.TrimSuffix(path, "/") + "/ws/agent"
		}
		return fmt.Sprintf("%s://%s%s", proto, u.Host, path)
	}

	// Helper to extract host & protocol from request, allowing explicit override via ?server= parameter
	getServerEndpoints := func(r *http.Request) (string, string) {
		customServer := strings.TrimSpace(r.URL.Query().Get("server"))
		if customServer != "" {
			if !strings.Contains(customServer, "://") {
				customServer = "http://" + customServer
			}
			if u, err := url.Parse(customServer); err == nil && u.Host != "" {
				proto := u.Scheme
				if proto == "" {
					proto = "http"
				}
				wsProto := "ws"
				if proto == "https" || proto == "wss" {
					proto = "https"
					wsProto = "wss"
				}
				return fmt.Sprintf("%s://%s", proto, u.Host), fmt.Sprintf("%s://%s/ws/agent", wsProto, u.Host)
			}
		}

		host := r.Host
		proto := "http"
		wsProto := "ws"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "https"
			wsProto = "wss"
		}
		return fmt.Sprintf("%s://%s", proto, host), fmt.Sprintf("%s://%s/ws/agent", wsProto, host)
	}

	// Helper to resolve candidate endpoints for smart roaming (LAN first, then Public/Cloudflare)
	resolveEndpoints := func(r *http.Request) ([]string, string) {
		var list []string
		seen := make(map[string]bool)
		add := func(u string) {
			norm := normalizeWS(u)
			if norm != "" && !seen[norm] {
				seen[norm] = true
				list = append(list, norm)
			}
		}

		if epParam := r.URL.Query().Get("endpoints"); epParam != "" {
			for _, ep := range strings.Split(epParam, ",") {
				add(ep)
			}
		}
		if lanSrv := r.URL.Query().Get("lan_server"); lanSrv != "" {
			add(lanSrv)
		}
		if pubSrv := r.URL.Query().Get("public_server"); pubSrv != "" {
			add(pubSrv)
		}

		_, defWS := getServerEndpoints(r)
		if len(list) == 0 {
			add(defWS)
		}

		primary := defWS
		if len(list) > 0 {
			primary = list[0]
		}
		return list, primary
	}

	// GET /api/health: Lightweight health check for latency and Smart Roaming LAN prober
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"version": version.Get(),
			"time":    time.Now().Unix(),
		})
	})

	// GET /api/version: Returns dynamic version and build info
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(version.Info())
	})

	// GET /api/server/network: Discovers local network interfaces for Smart Roaming LAN configuration
	mux.HandleFunc("GET /api/server/network", func(w http.ResponseWriter, r *http.Request) {
		var ips []string
		var primary string
		addrs, err := net.InterfaceAddrs()
		if err == nil {
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ip4 := ipnet.IP.To4(); ip4 != nil {
						ipStr := ip4.String()
						ips = append(ips, ipStr)
						if primary == "" && ip4.IsPrivate() {
							primary = ipStr
						}
					}
				}
			}
		}
		if primary == "" && len(ips) > 0 {
			primary = ips[0]
		}
		if primary == "" {
			primary = "127.0.0.1"
		}

		port := "8080"
		if _, p, err := net.SplitHostPort(r.Host); err == nil && p != "" {
			port = p
		}

		lanWS := fmt.Sprintf("ws://%s:%s/ws/agent", primary, port)
		httpBase, wsURL := getServerEndpoints(r)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"primary_lan_ip": primary,
			"lan_ips":        ips,
			"port":           port,
			"lan_ws":         lanWS,
			"current_host":   r.Host,
			"current_http":   httpBase,
			"current_ws":     wsURL,
		})
	})

	// Serves compiled agent binaries with dynamic configuration overlay embedding
	mux.HandleFunc("GET /bin/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := filepath.Base(r.PathValue("file"))
		binPath := findBinaryPath(file)
		if binPath == "" {
			slog.Warn("requested binary not found on server", "file", file)
			http.Error(w, fmt.Sprintf("Binary %s not found on server", file), http.StatusNotFound)
			return
		}

		token := r.URL.Query().Get("token")
		group := r.URL.Query().Get("group")
		endpoints, wsURL := resolveEndpoints(r)

		// If this is an agent binary, bake in the server address, candidate endpoints, enrollment token, and group!
		if strings.HasPrefix(file, "gomesh-agent") {
			f, err := os.Open(binPath)
			if err != nil {
				http.Error(w, "failed opening binary", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			cfg := embeddedcfg.AgentConfig{
				Server:    wsURL,
				Endpoints: endpoints,
				Token:     token,
				Group:     group,
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file))
			if err := embeddedcfg.AppendConfigToBinary(f, w, cfg); err != nil {
				slog.Error("failed streaming embedded config binary", "error", err)
			}
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file))
		http.ServeFile(w, r, binPath)
	})

	// GET /api/download/installer/windows-bat: Direct download 1-click Windows .bat installer
	mux.HandleFunc("GET /api/download/installer/windows-bat", func(w http.ResponseWriter, r *http.Request) {
		httpBase, _ := getServerEndpoints(r)
		endpoints, wsURL := resolveEndpoints(r)
		token := r.URL.Query().Get("token")
		if token == "" {
			token = "default-token"
		}
		group := r.URL.Query().Get("group")
		if group == "" {
			group = "Default"
		}
		epsStr := strings.Join(endpoints, ",")

		batScript := fmt.Sprintf(`@echo off
echo ========================================================
echo     GoMesh Remote Agent Windows One-Click Installer    
echo ========================================================
echo Server URL : %s
echo Endpoints  : %s
echo Group      : %s
echo.
powershell -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri '%s/scripts/install-windows.ps1' -OutFile install.ps1; .\install.ps1 -ServerUrl '%s' -Endpoints '%s' -Token '%s' -Group '%s'"
if %%ERRORLEVEL%% EQU 0 (
    echo.
    echo [+] GoMesh Agent installed and service started successfully in background!
) else (
    echo.
    echo [!] Installation failed. Please ensure you right-clicked and ran as Administrator.
)
pause
`, wsURL, epsStr, group, httpBase, wsURL, epsStr, token, group)

		w.Header().Set("Content-Type", "application/x-bat")
		w.Header().Set("Content-Disposition", "attachment; filename=\"install-gomesh.bat\"")
		w.Write([]byte(batScript))
	})

	// GET /api/download/installer/linux-sh: Direct download 1-click Linux installer
	mux.HandleFunc("GET /api/download/installer/linux-sh", func(w http.ResponseWriter, r *http.Request) {
		httpBase, _ := getServerEndpoints(r)
		endpoints, wsURL := resolveEndpoints(r)
		token := r.URL.Query().Get("token")
		if token == "" {
			token = "default-token"
		}
		group := r.URL.Query().Get("group")
		if group == "" {
			group = "Default"
		}
		epsStr := strings.Join(endpoints, ",")

		shScript := fmt.Sprintf(`#!/bin/bash
# GoMesh Remote Agent Linux One-Click Installer
set -e

if [ "$(id -u)" -ne 0 ]; then
    echo "[!] Error: Root or sudo privileges required. Run with: sudo bash install-gomesh.sh"
    exit 1
fi

echo "========================================================"
echo "    GoMesh Remote Agent Linux One-Click Installer      "
echo "========================================================"
curl -sSL '%s/scripts/install-linux.sh' | bash -s -- -s '%s' -e '%s' -t '%s' -g '%s'
`, httpBase, wsURL, epsStr, token, group)

		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"install-gomesh.sh\"")
		w.Write([]byte(shScript))
	})

	// GET /api/download/installer/macos-sh: Direct download 1-click macOS installer
	mux.HandleFunc("GET /api/download/installer/macos-sh", func(w http.ResponseWriter, r *http.Request) {
		httpBase, _ := getServerEndpoints(r)
		endpoints, wsURL := resolveEndpoints(r)
		token := r.URL.Query().Get("token")
		if token == "" {
			token = "default-token"
		}
		group := r.URL.Query().Get("group")
		if group == "" {
			group = "Default"
		}
		epsStr := strings.Join(endpoints, ",")

		shScript := fmt.Sprintf(`#!/bin/bash
# GoMesh Remote Agent macOS One-Click Installer
set -e

if [ "$(id -u)" -ne 0 ]; then
    echo "[!] Error: Root or sudo privileges required. Run with: sudo bash install-gomesh-macos.sh"
    exit 1
fi

echo "========================================================"
echo "    GoMesh Remote Agent macOS One-Click Installer      "
echo "========================================================"
curl -sSL '%s/scripts/install-macos.sh' | bash -s -- -s '%s' -e '%s' -t '%s' -g '%s'
`, httpBase, wsURL, epsStr, token, group)

		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"install-gomesh-macos.sh\"")
		w.Write([]byte(shScript))
	})

	// --- AUTH REST ENDPOINTS ---

	// POST /api/auth/login
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}

		user, err := st.AuthenticateUser(r.Context(), req.Username, req.Password)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid username or password"})
			return
		}

		// If 2FA is active, prompt for 6-digit TOTP code
		if user.TOTPEnabled {
			tempToken := sm.CreatePending2FA(user)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":      false,
				"requires_2fa": true,
				"temp_token":   tempToken,
				"message":      "Masukkan 6-digit kode OTP dari aplikasi Authenticator",
			})
			return
		}

		sess := sm.Create(user)
		auth.SetCookie(w, sess.ID, 24*time.Hour)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   sess.ID,
			"user": map[string]interface{}{
				"id":           user.ID,
				"username":     user.Username,
				"role":         user.Role,
				"totp_enabled": user.TOTPEnabled,
			},
		})
	})

	// POST /api/auth/login/2fa: Verifies 6-digit TOTP code to complete authentication
	mux.HandleFunc("POST /api/auth/login/2fa", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TempToken string `json:"temp_token"`
			Code      string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}

		user, ok := sm.GetPending2FA(req.TempToken)
		if !ok || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Sesi 2FA telah kadaluwarsa atau tidak valid. Silakan login kembali."})
			return
		}

		if !auth.ValidateTOTP(user.TOTPSecret, req.Code) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Kode 2FA tidak valid atau salah. Silakan coba lagi."})
			return
		}

		// Consume the pending token on success
		sm.ConsumePending2FA(req.TempToken)

		sess := sm.Create(user)
		auth.SetCookie(w, sess.ID, 24*time.Hour)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   sess.ID,
			"user": map[string]interface{}{
				"id":           user.ID,
				"username":     user.Username,
				"role":         user.Role,
				"totp_enabled": user.TOTPEnabled,
			},
		})
	})

	// POST /api/auth/logout
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if sess, ok := sm.GetSessionFromRequest(r); ok {
			sm.Revoke(sess.ID)
		}
		auth.ClearCookie(w)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	// GET /api/auth/me
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		sess, ok := sm.GetSessionFromRequest(r)
		if !ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
			return
		}
		user, _ := st.GetUserByID(r.Context(), sess.UserID)
		totpEnabled := false
		if user != nil {
			totpEnabled = user.TOTPEnabled
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"user": map[string]interface{}{
				"id":           sess.UserID,
				"username":     sess.Username,
				"role":         sess.Role,
				"totp_enabled": totpEnabled,
			},
		})
	})

	// --- 2FA MANAGEMENT REST ENDPOINTS (PROTECTED) ---

	// GET /api/auth/2fa/status
	mux.HandleFunc("GET /api/auth/2fa/status", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := auth.FromContext(r.Context())
		user, err := st.GetUserByID(r.Context(), sess.UserID)
		if err != nil || user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": user.TOTPEnabled,
		})
	}))

	// POST /api/auth/2fa/setup: Generates new TOTP secret, otpauth URL, and offline QR Code data URI
	mux.HandleFunc("POST /api/auth/2fa/setup", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := auth.FromContext(r.Context())
		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		otpURL := auth.GenerateOTPAuthURL("GoMesh", sess.Username, secret)
		qrDataURI, err := auth.GenerateQRCodePNG(otpURL, 256)
		if err != nil {
			slog.Warn("failed generating qr code png", "error", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"secret":      secret,
			"otpauth_url": otpURL,
			"qr_url":      qrDataURI,
		})
	}))

	// POST /api/auth/2fa/enable: Verifies code and activates 2FA
	mux.HandleFunc("POST /api/auth/2fa/enable", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := auth.FromContext(r.Context())
		var req struct {
			Secret string `json:"secret"`
			Code   string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if !auth.ValidateTOTP(req.Secret, req.Code) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Kode verifikasi 2FA tidak valid atau salah."})
			return
		}

		if err := st.UpdateUserTOTP(r.Context(), sess.UserID, req.Secret, true); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))

	// POST /api/auth/2fa/disable: Turns off 2FA
	mux.HandleFunc("POST /api/auth/2fa/disable", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := auth.FromContext(r.Context())
		var req struct {
			Password string `json:"password"`
			Code     string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		user, err := st.AuthenticateUser(r.Context(), sess.Username, req.Password)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Password konfirmasi salah."})
			return
		}

		if user.TOTPEnabled && req.Code != "" {
			if !auth.ValidateTOTP(user.TOTPSecret, req.Code) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Kode 2FA salah."})
				return
			}
		}

		if err := st.UpdateUserTOTP(r.Context(), sess.UserID, "", false); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))

	// POST /api/auth/password: Changes password for currently logged-in user
	mux.HandleFunc("POST /api/auth/password", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := auth.FromContext(r.Context())
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if len(req.NewPassword) < 6 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Password baru minimal 6 karakter."})
			return
		}

		user, err := st.AuthenticateUser(r.Context(), sess.Username, req.CurrentPassword)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Password saat ini salah."})
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed hashing password", http.StatusInternalServerError)
			return
		}

		if err := st.UpdateUserPassword(r.Context(), user.ID, string(newHash)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Password berhasil diperbarui"})
	}))

	// --- PROTECTED DEVICE & TELEMETRY ENDPOINTS ---

	// API: List Devices (Protected)
	mux.HandleFunc("GET /api/devices", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		devices, err := st.ListDevices(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("Error fetching devices: %v", err), http.StatusInternalServerError)
			return
		}
		if devices == nil {
			devices = make([]*store.Device, 0)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(devices)
	}))

	// API: Update Device Group (Protected)
	mux.HandleFunc("POST /api/devices/{id}/group", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Group string `json:"group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Group == "" {
			req.Group = "Default"
		}
		if err := st.UpdateDeviceGroup(r.Context(), id, req.Group); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "group": req.Group})
	}))

	mux.HandleFunc("DELETE /api/devices/{id}", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing device id", http.StatusBadRequest)
			return
		}

		h.DisconnectDevice(id)
		if err := st.DeleteDevice(r.Context(), id); err != nil {
			http.Error(w, fmt.Sprintf("failed to delete device: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "deleted_id": id})
	}))

	// API: List Device Groups (Protected)
	mux.HandleFunc("GET /api/groups", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		groups, err := st.ListDeviceGroups(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if groups == nil {
			groups = []string{"Default"}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	}))

	// WebSocket endpoint for Agents (Reverse connection - agent authenticates via token in HelloPayload)
	mux.HandleFunc("GET /ws/agent", func(w http.ResponseWriter, r *http.Request) {
		h.HandleAgentWS(w, r)
	})

	// WebSocket endpoint for Interactive Terminal (Protected)
	mux.HandleFunc("GET /ws/terminal", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.HandleTerminalWS(w, r)
	}))

	// WebSocket endpoint for Live Remote Desktop (Protected)
	mux.HandleFunc("GET /ws/desktop", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.HandleDesktopWS(w, r)
	}))

	// --- PROTECTED FILE MANAGER ENDPOINTS ---

	// API: List remote files
	mux.HandleFunc("GET /api/devices/{id}/files", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		targetPath := r.URL.Query().Get("path")
		resp, err := h.ListFiles(r.Context(), id, targetPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))

	// API: Download remote file
	mux.HandleFunc("GET /api/devices/{id}/files/download", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}

		baseName := filepath.Base(filePath)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", baseName))
		w.Header().Set("Content-Type", "application/octet-stream")

		offset := int64(0)
		chunkSize := 64 * 1024
		for {
			chunk, err := h.DownloadFileChunk(r.Context(), id, filePath, offset, chunkSize)
			if err != nil {
				slog.Error("error downloading chunk", "error", err)
				return
			}

			data, err := base64.StdEncoding.DecodeString(chunk.Data)
			if err != nil {
				return
			}
			w.Write(data)

			if chunk.IsEOF || len(data) == 0 {
				break
			}
			offset += int64(len(data))
		}
	}))

	// API: Upload file to remote agent
	mux.HandleFunc("POST /api/devices/{id}/files/upload", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		targetDir := r.URL.Query().Get("path")

		// 32MB max memory buffer
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "failed parsing multipart form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file in form", http.StatusBadRequest)
			return
		}
		defer file.Close()

		targetFilePath := filepath.Join(targetDir, header.Filename)
		offset := int64(0)
		buf := make([]byte, 64*1024)

		for {
			n, readErr := file.Read(buf)
			if n > 0 {
				isLast := readErr == io.EOF
				b64 := base64.StdEncoding.EncodeToString(buf[:n])
				if err := h.UploadFileChunk(r.Context(), id, targetFilePath, offset, b64, isLast); err != nil {
					http.Error(w, fmt.Sprintf("upload failed: %v", err), http.StatusInternalServerError)
					return
				}
				offset += int64(n)
			}
			if readErr != nil {
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "path": targetFilePath})
	}))

	// API: File actions (delete, mkdir)
	mux.HandleFunc("POST /api/devices/{id}/files/action", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Action string `json:"action"`
			Path   string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.FileAction(r.Context(), id, body.Action, body.Path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))

	// --- ENROLLMENT TOKEN MANAGEMENT ENDPOINTS ---

	// GET /api/tokens (Protected)
	mux.HandleFunc("GET /api/tokens", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		tokens, err := st.ListEnrollmentTokens(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if tokens == nil {
			tokens = make([]*store.EnrollmentToken, 0)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokens)
	}))

	// POST /api/tokens (Admin only)
	mux.HandleFunc("POST /api/tokens", sm.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Description string `json:"description"`
			ExpiresDays int    `json:"expires_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		currentUser, _ := auth.FromContext(r.Context())
		username := "admin"
		if currentUser != nil {
			username = currentUser.Username
		}

		token, err := st.CreateEnrollmentToken(r.Context(), req.Description, username, req.ExpiresDays)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)
	}))

	// --- AUDIT LOGS ENDPOINTS ---

	// GET /api/audits (Protected)
	mux.HandleFunc("GET /api/audits", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		audits, err := st.ListAudits(r.Context(), 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if audits == nil {
			audits = make([]*store.SessionAudit, 0)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(audits)
	}))

	return mux
}

// findBinaryPath locates an agent binary on local disk, or auto-fetches & caches it from GitHub Releases
func findBinaryPath(file string) string {
	candidates := []string{
		filepath.Join("bin", file),
		file,
		filepath.Join("..", "bin", file),
	}

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, file),
			filepath.Join(execDir, "bin", file),
			filepath.Join(execDir, "..", "bin", file),
		)
	}

	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	// Auto-fetch on demand from GitHub Releases if missing on disk
	return fetchAndCacheBinary(file)
}

func fetchAndCacheBinary(file string) string {
	targetDir := "bin"
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		targetDir = filepath.Join(execDir, "bin")
	}
	_ = os.MkdirAll(targetDir, 0755)
	destPath := filepath.Join(targetDir, file)

	urls := []string{
		fmt.Sprintf("https://github.com/didikari/gomesh/releases/latest/download/%s", file),
	}
	switch file {
	case "gomesh-agent.exe":
		urls = append(urls, "https://github.com/didikari/gomesh/releases/latest/download/gomesh-agent-windows-amd64.exe")
	case "gomesh-agent-linux":
		urls = append(urls, "https://github.com/didikari/gomesh/releases/latest/download/gomesh-agent-linux-amd64")
	case "gomesh-agent-linux-arm64":
		urls = append(urls, "https://github.com/didikari/gomesh/releases/latest/download/gomesh-agent-linux-arm64")
	case "gomesh-agent":
		urls = append(urls, "https://github.com/didikari/gomesh/releases/latest/download/gomesh-agent-darwin-arm64")
	case "gomesh-agent-mac-amd64":
		urls = append(urls, "https://github.com/didikari/gomesh/releases/latest/download/gomesh-agent-darwin-amd64")
	}

	client := &http.Client{Timeout: 90 * time.Second}
	for _, u := range urls {
		slog.Info("downloading agent binary from GitHub Releases", "file", file, "url", u)
		resp, err := client.Get(u)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		tmpFile := destPath + ".tmp"
		f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			resp.Body.Close()
			continue
		}
		_, copyErr := io.Copy(f, resp.Body)
		f.Close()
		resp.Body.Close()

		if copyErr == nil {
			if err := os.Rename(tmpFile, destPath); err == nil {
				slog.Info("successfully cached agent binary", "file", file, "path", destPath)
				return destPath
			}
		}
		_ = os.Remove(tmpFile)
	}

	return ""
}

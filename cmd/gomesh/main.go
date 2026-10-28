package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gomesh/internal/server/auth"
	"gomesh/internal/server/hub"
	"gomesh/internal/server/store"
	"gomesh/web"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server listening port")
	dbPath := flag.String("db", "data/gomesh.db", "Path to SQLite database file")
	flag.Parse()

	// Structured text logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting GoMesh Central Server", "port", *port, "db", *dbPath)

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

	mux := http.NewServeMux()

	// Web console UI
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		f, err := web.Assets.Open("index.html")
		if err != nil {
			http.Error(w, "Asset not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		io.Copy(w, f)
	})

	// Serves install scripts
	mux.HandleFunc("GET /scripts/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := filepath.Base(r.PathValue("file"))
		scriptPath := filepath.Join("scripts", file)
		data, err := os.ReadFile(scriptPath)
		if err != nil {
			http.Error(w, "Script not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})

	// Serves compiled agent binaries for installer downloads
	mux.HandleFunc("GET /bin/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := filepath.Base(r.PathValue("file"))
		binPath := filepath.Join("bin", file)
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			http.Error(w, "Binary not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, binPath)
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

		sess := sm.Create(user)
		auth.SetCookie(w, sess.ID, 24*time.Hour)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   sess.ID,
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"user": map[string]interface{}{
				"id":       sess.UserID,
				"username": sess.Username,
				"role":     sess.Role,
			},
		})
	})

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

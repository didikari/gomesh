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
		io.Copy(w, f)
	})

	// API: List Devices
	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// WebSocket endpoint for Agents (Reverse connection)
	mux.HandleFunc("GET /ws/agent", func(w http.ResponseWriter, r *http.Request) {
		h.HandleAgentWS(w, r)
	})

	// WebSocket endpoint for Interactive Terminal (Technician Web Console)
	mux.HandleFunc("GET /ws/terminal", func(w http.ResponseWriter, r *http.Request) {
		h.HandleTerminalWS(w, r)
	})

	// API: List remote files
	mux.HandleFunc("GET /api/devices/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		targetPath := r.URL.Query().Get("path")
		resp, err := h.ListFiles(r.Context(), id, targetPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// API: Download remote file
	mux.HandleFunc("GET /api/devices/{id}/files/download", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// API: Upload file to remote agent
	mux.HandleFunc("POST /api/devices/{id}/files/upload", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// API: File actions (delete, mkdir)
	mux.HandleFunc("POST /api/devices/{id}/files/action", func(w http.ResponseWriter, r *http.Request) {
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
	})

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

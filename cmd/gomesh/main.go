package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

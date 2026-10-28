package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gomesh/internal/agent"
)

func main() {
	serverURL := flag.String("server", "ws://localhost:8080/ws/agent", "GoMesh Central Server WebSocket URL")
	token := flag.String("token", "", "GoMesh Enrollment Token")
	configDir := flag.String("config", ".", "Path to store agent persistent identity")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	client, err := agent.NewClient(*serverURL, *configDir)
	if err != nil {
		slog.Error("failed to create agent client", "error", err)
		os.Exit(1)
	}

	if *token != "" {
		client.SetAuthToken(*token)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("received termination signal, shutting down agent...")
		cancel()
	}()

	if err := client.Run(ctx); err != nil {
		slog.Error("agent exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("GoMesh Agent stopped cleanly")
}

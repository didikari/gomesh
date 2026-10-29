package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gomesh/internal/agent"
	"gomesh/internal/agent/embeddedcfg"
	"gomesh/internal/version"
)

func main() {
	serverURL := flag.String("server", "", "GoMesh Central Server WebSocket URL")
	endpointsFlag := flag.String("endpoints", "", "Comma-separated list of GoMesh server endpoints (Priority ordered, e.g. LAN then Public)")
	token := flag.String("token", "", "GoMesh Enrollment Token")
	group := flag.String("group", "", "Target Device Group Name (e.g. Group A, Office, Lab)")
	configDir := flag.String("config", ".", "Path to store agent persistent identity")
	foreground := flag.Bool("foreground", false, "Run in foreground/console mode (disable self-installation and console hiding)")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall and stop all GoMesh background agent instances")
	showVersion := flag.Bool("version", false, "Print agent version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("GoMesh Agent %s\n", version.Get())
		return
	}

	if *uninstallFlag {
		uninstallAgent()
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting GoMesh Agent", "version", version.Get())

	// 1. Read baked-in configuration from executable overlay if present
	embedded := embeddedcfg.ReadFromCurrentExecutable()

	resolvedServer := *serverURL
	resolvedToken := *token
	resolvedGroup := *group

	if resolvedServer == "" && embedded != nil && embedded.Server != "" {
		resolvedServer = embedded.Server
		slog.Info("loaded embedded server configuration", "server", resolvedServer)
	}
	if resolvedToken == "" && embedded != nil && embedded.Token != "" {
		resolvedToken = embedded.Token
		slog.Info("loaded embedded enrollment token")
	}
	if resolvedGroup == "" && embedded != nil && embedded.Group != "" {
		resolvedGroup = embedded.Group
		slog.Info("loaded embedded target group", "group", resolvedGroup)
	}

	// 2. Resolve Server Endpoints for Smart Roaming
	var resolvedEndpoints []string
	if *endpointsFlag != "" {
		for _, ep := range strings.Split(*endpointsFlag, ",") {
			if trimmed := strings.TrimSpace(ep); trimmed != "" {
				resolvedEndpoints = append(resolvedEndpoints, trimmed)
			}
		}
	} else if embedded != nil && len(embedded.GetEndpoints()) > 0 {
		resolvedEndpoints = embedded.GetEndpoints()
	}

	if len(resolvedEndpoints) > 0 && resolvedServer == "" {
		resolvedServer = resolvedEndpoints[0]
	}

	// Default fallback if neither flag nor embedded config was supplied
	if resolvedServer == "" {
		resolvedServer = "ws://localhost:8080/ws/agent"
	}
	if len(resolvedEndpoints) == 0 {
		resolvedEndpoints = []string{resolvedServer}
	}

	client, err := agent.NewClient(resolvedServer, *configDir)
	if err != nil {
		slog.Error("failed to create agent client", "error", err)
		os.Exit(1)
	}

	client.SetEndpoints(resolvedEndpoints)
	slog.Info("configured server endpoints for smart roaming", "endpoints", resolvedEndpoints)

	if resolvedToken != "" {
		client.SetAuthToken(resolvedToken)
	}
	if resolvedGroup != "" {
		client.SetGroupName(resolvedGroup)
	}

	// 1. If running as a registered Windows Service, delegate execution to Service Control Manager
	handled, svcErr := runAsWindowsService("GoMeshAgent", client.Run)
	if handled {
		if svcErr != nil {
			slog.Error("windows service terminated with error", "error", svcErr)
			os.Exit(1)
		}
		slog.Info("GoMesh Agent Windows Service stopped cleanly")
		return
	}

	isCLI := *foreground || *serverURL != "" || *token != ""
	if checkAndSelfInstall(false, isCLI) {
		return
	}

	acquired, unlock := acquireSingleInstanceLock()
	if !acquired {
		slog.Warn("another instance of GoMesh Agent is already running on this machine, exiting cleanly")
		return
	}
	defer unlock()

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

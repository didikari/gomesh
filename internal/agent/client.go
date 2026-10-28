package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"gomesh/internal/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Config holds local agent settings and persistent identity
type Config struct {
	DeviceID     string `json:"device_id"`
	HardwareUUID string `json:"hardware_uuid"`
	ServerURL    string `json:"server_url"`
	AuthToken    string `json:"auth_token,omitempty"`
}

// Client represents the GoMesh background device agent
type Client struct {
	cfg       Config
	cfgPath   string
	version   string
	startTime time.Time
	mu        sync.Mutex
}

// NewClient initializes a client with persistent configuration
func NewClient(serverURL string, configDir string) (*Client, error) {
	if configDir == "" {
		configDir = "."
	}
	cfgPath := filepath.Join(configDir, "gomesh-agent.json")

	cfg := Config{
		ServerURL: serverURL,
	}

	// Try reading existing config
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	// Ensure persistent UUID exists
	if cfg.HardwareUUID == "" {
		cfg.HardwareUUID = uuid.NewString()
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = uuid.NewString()
	}
	cfg.ServerURL = serverURL

	// Save back config
	if b, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = os.WriteFile(cfgPath, b, 0644)
	}

	return &Client{
		cfg:       cfg,
		cfgPath:   cfgPath,
		version:   "0.1.0",
		startTime: time.Now(),
	}, nil
}

// Run starts the persistent outbound agent loop
func (c *Client) Run(ctx context.Context) error {
	backoff := 1 * time.Second
	maxBackoff := 15 * time.Second

	hostname, _ := os.Hostname()
	internalIP := getInternalIP()

	slog.Info("GoMesh Agent initialized",
		"device_id", c.cfg.DeviceID,
		"hardware_uuid", c.cfg.HardwareUUID,
		"hostname", hostname,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"internal_ip", internalIP,
		"server", c.cfg.ServerURL,
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("agent stopping per context cancellation")
			return nil
		default:
		}

		slog.Info("connecting to GoMesh Central Server...", "url", c.cfg.ServerURL)
		err := c.connectAndServe(ctx)
		if err != nil {
			slog.Warn("connection ended with error", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// connectAndServe handles a single connection session
func (c *Client) connectAndServe(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.DialContext(ctx, c.cfg.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("failed dial websocket: %w", err)
	}
	defer conn.Close()

	hostname, _ := os.Hostname()

	// 1. Send Hello handshake
	hello := protocol.HelloPayload{
		DeviceID:     c.cfg.DeviceID,
		HardwareUUID: c.cfg.HardwareUUID,
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		InternalIP:   getInternalIP(),
		AgentVersion: c.version,
		Token:        c.cfg.AuthToken,
	}

	if err := conn.WriteJSON(protocol.NewEnvelope(protocol.TypeHello, hello)); err != nil {
		return fmt.Errorf("failed sending hello envelope: %w", err)
	}

	// 2. Wait for HelloAck
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var ackEnv protocol.Envelope
	if err := conn.ReadJSON(&ackEnv); err != nil {
		return fmt.Errorf("failed receiving hello ack: %w", err)
	}
	if ackEnv.Type != protocol.TypeHelloAck {
		return fmt.Errorf("unexpected ack response type: %s", ackEnv.Type)
	}

	slog.Info("registered successfully with GoMesh Central Server!")

	// 3. Start Heartbeat Loop
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	defer close(done)

	// Goroutine to send heartbeats
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				// Lightweight memory calculation (MB)
				ramMB := float64(m.Alloc) / 1024 / 1024

				// Estimated CPU variation (mocked realistic telemetry for MVP)
				cpuVal := 1.0 + rand.Float64()*3.0

				hb := protocol.HeartbeatPayload{
					DeviceID: c.cfg.DeviceID,
					CPUUsage: cpuVal,
					RAMUsage: ramMB,
					Uptime:   uint64(time.Since(c.startTime).Seconds()),
				}

				c.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				err := conn.WriteJSON(protocol.NewEnvelope(protocol.TypeHeartbeat, hb))
				c.mu.Unlock()

				if err != nil {
					slog.Warn("failed to send heartbeat", "error", err)
					conn.Close()
					return
				}
			}
		}
	}()

	// Read loop to receive server commands
	for {
		conn.SetReadDeadline(time.Now().Add(40 * time.Second))
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		switch env.Type {
		case protocol.TypeHeartbeatAck:
			slog.Debug("heartbeat acknowledged by server")
		default:
			slog.Info("received server command", "type", env.Type)
		}
	}
}

// getInternalIP returns the primary local network IP
func getInternalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

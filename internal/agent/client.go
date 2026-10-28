package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gomesh/internal/agent/desktop"
	"gomesh/internal/agent/filemgr"
	"gomesh/internal/agent/pty"
	"gomesh/internal/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var errSessionDropped = errors.New("active session dropped")

// Config holds local agent settings and persistent identity
type Config struct {
	DeviceID     string   `json:"device_id"`
	HardwareUUID string   `json:"hardware_uuid"`
	ServerURL    string   `json:"server_url"`
	Endpoints    []string `json:"endpoints,omitempty"`
	AuthToken    string   `json:"auth_token,omitempty"`
	GroupName    string   `json:"group_name,omitempty"`
}

// Client represents the GoMesh background device agent
type Client struct {
	cfg         Config
	cfgPath     string
	version     string
	startTime   time.Time
	mu          sync.Mutex
	ptySessions map[string]pty.PTY
	ptyMu       sync.Mutex
	desktopStop chan struct{}
	desktopMu   sync.Mutex
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
		cfg:         cfg,
		cfgPath:     cfgPath,
		version:     "0.1.0",
		startTime:   time.Now(),
		ptySessions: make(map[string]pty.PTY),
	}, nil
}

// SetAuthToken configures the enrollment or authentication token
func (c *Client) SetAuthToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.AuthToken = token
	if b, err := json.MarshalIndent(c.cfg, "", "  "); err == nil {
		_ = os.WriteFile(c.cfgPath, b, 0644)
	}
}

// SetGroupName configures the target machine group name
func (c *Client) SetGroupName(group string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.GroupName = group
	if b, err := json.MarshalIndent(c.cfg, "", "  "); err == nil {
		_ = os.WriteFile(c.cfgPath, b, 0644)
	}
}

// SetEndpoints configures candidate endpoints in order of priority (e.g. LAN first, then Public/Cloudflare)
func (c *Client) SetEndpoints(endpoints []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.Endpoints = endpoints
	if b, err := json.MarshalIndent(c.cfg, "", "  "); err == nil {
		_ = os.WriteFile(c.cfgPath, b, 0644)
	}
}

// GetEndpoints returns an ordered, deduplicated list of server endpoints to attempt
func (c *Client) GetEndpoints() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var list []string
	seen := make(map[string]bool)
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			list = append(list, u)
		}
	}
	for _, ep := range c.cfg.Endpoints {
		add(ep)
	}
	if c.cfg.ServerURL != "" {
		add(c.cfg.ServerURL)
	}
	if len(list) == 0 {
		list = []string{"ws://localhost:8080/ws/agent"}
	}
	return list
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
		"endpoints", c.GetEndpoints(),
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("agent stopping per context cancellation")
			c.closeAllPTY()
			return nil
		default:
		}

		endpoints := c.GetEndpoints()
		var connectedOnce bool

		for i, ep := range endpoints {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			isPrimary := (i == 0)
			modeLabel := "LAN"
			if !isPrimary {
				modeLabel = "Internet/WAN"
			}
			slog.Info("connecting to GoMesh Server...", "endpoint", ep, "priority", i+1, "mode", modeLabel)

			err := c.connectAndServe(ctx, ep, isPrimary, endpoints)
			if err == nil {
				connectedOnce = true
				backoff = 1 * time.Second
				break
			}

			if errors.Is(err, errSessionDropped) {
				slog.Info("connection to server ended, rediscovering endpoints from priority 1...", "last_endpoint", ep)
				connectedOnce = true
				backoff = 1 * time.Second
				break
			}

			slog.Info("endpoint unreachable, trying fallback...", "endpoint", ep, "error", err)
		}

		// Ensure all PTY processes close when connection drops
		c.closeAllPTY()

		if !connectedOnce {
			slog.Warn("all server endpoints failed to connect, backing off...", "backoff", backoff)
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
}

func (c *Client) sendEnvelope(conn *websocket.Conn, env protocol.Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteJSON(env)
}

// toHealthURL converts a websocket server URL to an HTTP health check URL
func toHealthURL(wsURL string) string {
	u, err := url.Parse(wsURL)
	if err != nil {
		return ""
	}
	scheme := "http"
	if u.Scheme == "wss" || u.Scheme == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/health", scheme, u.Host)
}

// connectAndServe handles a single connection session to a target endpoint
func (c *Client) connectAndServe(ctx context.Context, endpoint string, isPrimary bool, allEndpoints []string) error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 3 * time.Second

	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed dial websocket: %w", err)
	}
	defer conn.Close()

	hostname, _ := os.Hostname()

	connMode := "WAN"
	if isPrimary {
		connMode = "LAN"
	}

	// 1. Send Hello handshake
	hello := protocol.HelloPayload{
		DeviceID:          c.cfg.DeviceID,
		HardwareUUID:      c.cfg.HardwareUUID,
		Hostname:          hostname,
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		InternalIP:        getInternalIP(),
		AgentVersion:      c.version,
		Token:             c.cfg.AuthToken,
		GroupName:         c.cfg.GroupName,
		ConnectionMode:    connMode,
		ConnectedEndpoint: endpoint,
	}

	if err := c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypeHello, hello)); err != nil {
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

	// Goroutine to close socket immediately when context is canceled
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
			conn.Close()
		}
	}()

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
				ramMB := float64(m.Alloc) / 1024 / 1024
				cpuVal := 1.0 + rand.Float64()*3.0

				hb := protocol.HeartbeatPayload{
					DeviceID: c.cfg.DeviceID,
					CPUUsage: cpuVal,
					RAMUsage: ramMB,
					Uptime:   uint64(time.Since(c.startTime).Seconds()),
				}

				if err := c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypeHeartbeat, hb)); err != nil {
					slog.Warn("failed to send heartbeat", "error", err)
					conn.Close()
					return
				}
			}
		}
	}()

	// 4. Background prober: if connected via fallback (WAN), probe primary LAN endpoint
	if !isPrimary && len(allEndpoints) > 0 {
		lanEP := allEndpoints[0]
		healthURL := toHealthURL(lanEP)
		if healthURL != "" {
			go func() {
				probeTicker := time.NewTicker(30 * time.Second)
				defer probeTicker.Stop()
				httpClient := &http.Client{Timeout: 1500 * time.Millisecond}
				for {
					select {
					case <-done:
						return
					case <-ctx.Done():
						return
					case <-probeTicker.C:
						resp, err := httpClient.Get(healthURL)
						if err == nil {
							resp.Body.Close()
							if resp.StatusCode == http.StatusOK {
								slog.Info("primary LAN endpoint is reachable! Switching from WAN to high-speed LAN...", "lan_endpoint", lanEP)
								conn.Close()
								return
							}
						}
					}
				}
			}()
		}
	}

	// Read loop to receive server commands
	for {
		conn.SetReadDeadline(time.Now().Add(40 * time.Second))
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return fmt.Errorf("%w: read error: %v", errSessionDropped, err)
		}

		switch env.Type {
		case protocol.TypeHeartbeatAck:
			slog.Debug("heartbeat acknowledged by server")

		case protocol.TypePTYStart:
			raw, _ := json.Marshal(env.Payload)
			var payload protocol.PTYStartPayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				go c.startPTYSession(conn, payload)
			}

		case protocol.TypePTYData:
			raw, _ := json.Marshal(env.Payload)
			var payload protocol.PTYDataPayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				c.writePTYData(payload.SessionID, payload.Data)
			}

		case protocol.TypePTYResize:
			raw, _ := json.Marshal(env.Payload)
			var payload protocol.PTYResizePayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				c.resizePTY(payload.SessionID, payload.Cols, payload.Rows)
			}

		case protocol.TypePTYStop:
			raw, _ := json.Marshal(env.Payload)
			var payload protocol.PTYStopPayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				c.stopPTYSession(payload.SessionID)
			}

		case protocol.TypeFileListReq:
			raw, _ := json.Marshal(env.Payload)
			var req protocol.FileListReqPayload
			if err := json.Unmarshal(raw, &req); err == nil {
				go func(r protocol.FileListReqPayload) {
					resolvedPath, entries, err := filemgr.List(r.Path)
					errMsg := ""
					if err != nil {
						errMsg = err.Error()
					}
					resp := protocol.FileListRespPayload{
						RequestID: r.RequestID,
						Path:      resolvedPath,
						Entries:   entries,
						Error:     errMsg,
					}
					_ = c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypeFileListResp, resp))
				}(req)
			}

		case protocol.TypeFileDownloadReq:
			raw, _ := json.Marshal(env.Payload)
			var req protocol.FileDownloadReqPayload
			if err := json.Unmarshal(raw, &req); err == nil {
				go func(r protocol.FileDownloadReqPayload) {
					chunkData, isEOF, err := filemgr.ReadChunk(r.Path, r.Offset, r.ChunkSize)
					errMsg := ""
					if err != nil {
						errMsg = err.Error()
					}
					resp := protocol.FileDownloadChunkPayload{
						RequestID: r.RequestID,
						Offset:    r.Offset,
						Data:      chunkData,
						IsEOF:     isEOF,
						Error:     errMsg,
					}
					_ = c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypeFileDownloadChunk, resp))
				}(req)
			}

		case protocol.TypeFileUploadChunk:
			raw, _ := json.Marshal(env.Payload)
			var req protocol.FileUploadChunkPayload
			if err := json.Unmarshal(raw, &req); err == nil {
				go func(r protocol.FileUploadChunkPayload) {
					err := filemgr.WriteChunk(r.Path, r.Offset, r.Data, r.IsLast)
					errMsg := ""
					success := true
					if err != nil {
						errMsg = err.Error()
						success = false
					}
					resp := protocol.FileUploadRespPayload{
						RequestID: r.RequestID,
						Success:   success,
						Error:     errMsg,
					}
					_ = c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypeFileUploadResp, resp))
				}(req)
			}

		case protocol.TypeFileActionReq:
			raw, _ := json.Marshal(env.Payload)
			var req protocol.FileActionReqPayload
			if err := json.Unmarshal(raw, &req); err == nil {
				go func(r protocol.FileActionReqPayload) {
					err := filemgr.Action(r.Action, r.Path)
					errMsg := ""
					success := true
					if err != nil {
						errMsg = err.Error()
						success = false
					}
					resp := protocol.FileActionRespPayload{
						RequestID: r.RequestID,
						Success:   success,
						Error:     errMsg,
					}
					_ = c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypeFileActionResp, resp))
				}(req)
			}

		case protocol.TypeDesktopStart:
			raw, _ := json.Marshal(env.Payload)
			var payload protocol.DesktopStartPayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				c.startDesktopStream(conn, payload)
			}

		case protocol.TypeDesktopStop:
			c.stopDesktopStream()

		case protocol.TypeDesktopInput:
			raw, _ := json.Marshal(env.Payload)
			var payload protocol.DesktopInputPayload
			if err := json.Unmarshal(raw, &payload); err == nil {
				_ = desktop.HandleInput(payload)
			}

		default:
			slog.Info("received server command", "type", env.Type)
		}
	}
}

func (c *Client) startPTYSession(conn *websocket.Conn, payload protocol.PTYStartPayload) {
	slog.Info("starting new PTY session", "session_id", payload.SessionID, "cols", payload.Cols, "rows", payload.Rows)

	p, err := pty.Start(payload.Cols, payload.Rows)
	if err != nil {
		slog.Error("failed to start pty process", "error", err)
		_ = c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypePTYStop, protocol.PTYStopPayload{
			SessionID: payload.SessionID,
		}))
		return
	}

	c.ptyMu.Lock()
	c.ptySessions[payload.SessionID] = p
	c.ptyMu.Unlock()

	// Read output from PTY and stream to server
	buf := make([]byte, 4096)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			env := protocol.NewEnvelope(protocol.TypePTYData, protocol.PTYDataPayload{
				SessionID: payload.SessionID,
				Data:      data,
			})
			if sendErr := c.sendEnvelope(conn, env); sendErr != nil {
				break
			}
		}
		if err != nil {
			if err != io.EOF {
				slog.Debug("pty read ended", "error", err)
			}
			break
		}
	}

	c.stopPTYSession(payload.SessionID)
	_ = c.sendEnvelope(conn, protocol.NewEnvelope(protocol.TypePTYStop, protocol.PTYStopPayload{
		SessionID: payload.SessionID,
	}))
}

func (c *Client) writePTYData(sessionID string, data string) {
	c.ptyMu.Lock()
	p, ok := c.ptySessions[sessionID]
	c.ptyMu.Unlock()

	if ok && p != nil {
		_, _ = p.Write([]byte(data))
	}
}

func (c *Client) resizePTY(sessionID string, cols, rows uint16) {
	c.ptyMu.Lock()
	p, ok := c.ptySessions[sessionID]
	c.ptyMu.Unlock()

	if ok && p != nil {
		_ = p.Resize(cols, rows)
	}
}

func (c *Client) stopPTYSession(sessionID string) {
	c.ptyMu.Lock()
	defer c.ptyMu.Unlock()

	if p, ok := c.ptySessions[sessionID]; ok {
		_ = p.Close()
		delete(c.ptySessions, sessionID)
		slog.Info("PTY session terminated", "session_id", sessionID)
	}
}

func (c *Client) closeAllPTY() {
	c.ptyMu.Lock()
	defer c.ptyMu.Unlock()

	for id, p := range c.ptySessions {
		_ = p.Close()
		delete(c.ptySessions, id)
	}

	c.stopDesktopStream()
}

func (c *Client) startDesktopStream(conn *websocket.Conn, payload protocol.DesktopStartPayload) {
	c.desktopMu.Lock()
	defer c.desktopMu.Unlock()

	// Stop any existing desktop stream
	if c.desktopStop != nil {
		close(c.desktopStop)
	}

	stopChan := make(chan struct{})
	c.desktopStop = stopChan

	fps := payload.FPS
	if fps <= 0 || fps > 30 {
		fps = 15
	}
	interval := time.Duration(1000/fps) * time.Millisecond

	quality := payload.Quality
	if quality <= 0 || quality > 100 {
		quality = 65
	}

	slog.Info("starting desktop live screen capture", "session_id", payload.SessionID, "fps", fps, "quality", quality)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var lastErrLog time.Time
		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				frame, err := desktop.CaptureDisplay(payload.Display, quality)
				if err != nil {
					if time.Since(lastErrLog) > 5*time.Second {
						slog.Warn("desktop screen capture error", "error", err)
						lastErrLog = time.Now()
					}
					continue
				}
				frame.SessionID = payload.SessionID

				env := protocol.NewEnvelope(protocol.TypeDesktopFrame, frame)
				if err := c.sendEnvelope(conn, env); err != nil {
					return
				}
			}
		}
	}()
}

func (c *Client) stopDesktopStream() {
	c.desktopMu.Lock()
	defer c.desktopMu.Unlock()
	if c.desktopStop != nil {
		close(c.desktopStop)
		c.desktopStop = nil
		slog.Info("desktop stream stopped")
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

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gomesh/internal/protocol"
	"gomesh/internal/server/store"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for API & agent reverse connections
	},
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
}

// AgentSession represents an active outbound agent connection
type AgentSession struct {
	DeviceID      string
	HardwareUUID  string
	Conn          *websocket.Conn
	DeviceInfo    protocol.HelloPayload
	ConnectedAt   time.Time
	LastHeartbeat time.Time
	writeMu       sync.Mutex
	closed        bool
}

// SendMessage sends an envelope safely to this agent
func (s *AgentSession) SendMessage(env protocol.Envelope) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	s.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return s.Conn.WriteJSON(env)
}

// TerminalSession represents an active web browser terminal connection
type TerminalSession struct {
	SessionID   string
	DeviceID    string
	BrowserConn *websocket.Conn
	writeMu     sync.Mutex
	closed      bool
}

// Hub manages active agent and browser sessions
type Hub struct {
	store            *store.Store
	agents           map[string]*AgentSession          // keyed by DeviceID
	terminalSessions map[string]*TerminalSession       // keyed by SessionID
	pendingResponses map[string]chan protocol.Envelope // keyed by RequestID
	mu               sync.RWMutex
	termMu           sync.RWMutex
	respMu           sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// New creates and starts a new Hub
func New(st *store.Store) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		store:            st,
		agents:           make(map[string]*AgentSession),
		terminalSessions: make(map[string]*TerminalSession),
		pendingResponses: make(map[string]chan protocol.Envelope),
		ctx:              ctx,
		cancel:           cancel,
	}

	go h.reaperLoop()
	return h
}

// Stop shuts down the hub
func (h *Hub) Stop() {
	h.cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, a := range h.agents {
		a.writeMu.Lock()
		a.closed = true
		a.Conn.Close()
		a.writeMu.Unlock()
	}
	h.agents = make(map[string]*AgentSession)

	h.termMu.Lock()
	defer h.termMu.Unlock()
	for _, t := range h.terminalSessions {
		t.writeMu.Lock()
		t.closed = true
		t.BrowserConn.Close()
		t.writeMu.Unlock()
	}
	h.terminalSessions = make(map[string]*TerminalSession)
}

// reaperLoop periodically checks for stale connections
func (h *Hub) reaperLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			// Mark devices whose last heartbeat was > 30s ago as offline
			affected, err := h.store.MarkStaleDevices(h.ctx, 30*time.Second)
			if err != nil {
				slog.Error("error checking stale devices", "error", err)
			} else if affected > 0 {
				slog.Info("marked stale devices offline", "count", affected)
			}

			// Clean up in-memory sessions that haven't sent a heartbeat in > 45s
			h.mu.Lock()
			cutoff := time.Now().Add(-45 * time.Second)
			for id, sess := range h.agents {
				if sess.LastHeartbeat.Before(cutoff) {
					slog.Warn("closing inactive agent connection", "device_id", id)
					sess.writeMu.Lock()
					sess.closed = true
					sess.Conn.Close()
					sess.writeMu.Unlock()
					delete(h.agents, id)
				}
			}
			h.mu.Unlock()
		}
	}
}

// HandleAgentWS handles reverse WebSocket incoming connection from agents
func (h *Hub) HandleAgentWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	// Wait for Hello message within 10 seconds
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var helloEnv protocol.Envelope
	if err := conn.ReadJSON(&helloEnv); err != nil {
		slog.Warn("agent failed to send hello message", "error", err)
		return
	}

	if helloEnv.Type != protocol.TypeHello {
		slog.Warn("unexpected initial message type", "type", helloEnv.Type)
		return
	}

	rawPayload, err := json.Marshal(helloEnv.Payload)
	if err != nil {
		slog.Error("failed marshaling hello payload", "error", err)
		return
	}

	var hello protocol.HelloPayload
	if err := json.Unmarshal(rawPayload, &hello); err != nil {
		slog.Error("failed unmarshaling hello payload", "error", err)
		return
	}

	if hello.HardwareUUID == "" {
		slog.Warn("agent sent empty hardware UUID")
		return
	}

	if hello.DeviceID == "" {
		hello.DeviceID = uuid.NewString()
	}

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	dev := &store.Device{
		ID:           hello.DeviceID,
		HardwareUUID: hello.HardwareUUID,
		Hostname:     hello.Hostname,
		OS:           hello.OS,
		Arch:         hello.Arch,
		InternalIP:   hello.InternalIP,
		PublicIP:     clientIP,
		AgentVersion: hello.AgentVersion,
		Status:       "online",
	}

	if err := h.store.UpsertDevice(h.ctx, dev); err != nil {
		slog.Error("failed upserting device in store", "error", err)
		return
	}

	// Update with assigned ID from database
	hello.DeviceID = dev.ID

	session := &AgentSession{
		DeviceID:      dev.ID,
		HardwareUUID:  dev.HardwareUUID,
		Conn:          conn,
		DeviceInfo:    hello,
		ConnectedAt:   time.Now().UTC(),
		LastHeartbeat: time.Now().UTC(),
	}

	h.mu.Lock()
	// Close any existing connection for this device
	if old, exists := h.agents[dev.ID]; exists {
		old.writeMu.Lock()
		old.closed = true
		old.Conn.Close()
		old.writeMu.Unlock()
	}
	h.agents[dev.ID] = session
	h.mu.Unlock()

	slog.Info("agent connected successfully",
		"device_id", dev.ID,
		"hostname", dev.Hostname,
		"os", dev.OS,
		"arch", dev.Arch,
		"public_ip", clientIP,
	)

	// Send HelloAck
	ackEnv := protocol.NewEnvelope(protocol.TypeHelloAck, protocol.HelloAckPayload{
		Accepted:   true,
		Message:    "Welcome to GoMesh",
		ServerTime: time.Now().UnixMilli(),
	})
	if err := session.SendMessage(ackEnv); err != nil {
		slog.Error("failed sending hello ack", "error", err)
		return
	}

	defer func() {
		h.mu.Lock()
		delete(h.agents, dev.ID)
		h.mu.Unlock()

		h.store.SetDeviceStatus(h.ctx, dev.ID, "offline")
		slog.Info("agent disconnected", "device_id", dev.ID, "hostname", dev.Hostname)
	}()

	// Reset read deadline to 40s (heartbeat every 10s)
	conn.SetReadDeadline(time.Now().Add(40 * time.Second))

	// Main receive loop from agent
	for {
		var env protocol.Envelope
		err := conn.ReadJSON(&env)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("agent closed unexpectedly", "error", err)
			}
			break
		}

		conn.SetReadDeadline(time.Now().Add(40 * time.Second))

		switch env.Type {
		case protocol.TypeHeartbeat:
			session.LastHeartbeat = time.Now().UTC()
			raw, _ := json.Marshal(env.Payload)
			var hb protocol.HeartbeatPayload
			if err := json.Unmarshal(raw, &hb); err == nil {
				h.store.UpdateHeartbeat(h.ctx, dev.ID, hb.CPUUsage, hb.RAMUsage)
			}

			// Respond with HeartbeatAck
			ack := protocol.NewEnvelope(protocol.TypeHeartbeatAck, protocol.HeartbeatAckPayload{
				ReceivedAt: time.Now().UnixMilli(),
			})
			_ = session.SendMessage(ack)

		case protocol.TypePTYData:
			raw, _ := json.Marshal(env.Payload)
			var ptyData protocol.PTYDataPayload
			if err := json.Unmarshal(raw, &ptyData); err == nil {
				h.termMu.RLock()
				termSess, exists := h.terminalSessions[ptyData.SessionID]
				h.termMu.RUnlock()
				if exists && termSess != nil {
					termSess.writeMu.Lock()
					if !termSess.closed {
						termSess.BrowserConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
						_ = termSess.BrowserConn.WriteMessage(websocket.TextMessage, []byte(ptyData.Data))
					}
					termSess.writeMu.Unlock()
				}
			}

		case protocol.TypePTYStop:
			raw, _ := json.Marshal(env.Payload)
			var ptyStop protocol.PTYStopPayload
			if err := json.Unmarshal(raw, &ptyStop); err == nil {
				h.closeTerminalSession(ptyStop.SessionID)
			}

		case protocol.TypeFileListResp, protocol.TypeFileDownloadChunk, protocol.TypeFileUploadResp, protocol.TypeFileActionResp:
			raw, _ := json.Marshal(env.Payload)
			var header struct {
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(raw, &header); err == nil && header.RequestID != "" {
				h.respMu.RLock()
				ch, ok := h.pendingResponses[header.RequestID]
				h.respMu.RUnlock()
				if ok && ch != nil {
					select {
					case ch <- env:
					default:
					}
				}
			}

		default:
			slog.Debug("received message from agent", "type", env.Type, "device_id", dev.ID)
		}
	}
}

// HandleTerminalWS handles terminal sessions opened from technician's browser
func (h *Hub) HandleTerminalWS(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "missing device_id", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	agentSess, exists := h.agents[deviceID]
	h.mu.RUnlock()

	if !exists || agentSess == nil {
		http.Error(w, "device is not currently online", http.StatusNotFound)
		return
	}

	browserConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("terminal websocket upgrade failed", "error", err)
		return
	}
	defer browserConn.Close()

	sessionID := uuid.NewString()
	termSess := &TerminalSession{
		SessionID:   sessionID,
		DeviceID:    deviceID,
		BrowserConn: browserConn,
	}

	h.termMu.Lock()
	h.terminalSessions[sessionID] = termSess
	h.termMu.Unlock()

	defer h.closeTerminalSession(sessionID)

	slog.Info("opening interactive terminal session", "session_id", sessionID, "device_id", deviceID)

	// Send PTYStart to target agent
	cols := uint16(80)
	rows := uint16(24)
	startEnv := protocol.NewEnvelope(protocol.TypePTYStart, protocol.PTYStartPayload{
		SessionID: sessionID,
		Cols:      cols,
		Rows:      rows,
	})
	if err := agentSess.SendMessage(startEnv); err != nil {
		slog.Error("failed sending pty start to agent", "error", err)
		return
	}

	// Read input from browser (keystrokes / resize) and forward to agent
	for {
		msgType, msg, err := browserConn.ReadMessage()
		if err != nil {
			break
		}

		if msgType == websocket.TextMessage {
			// Check if message is a resize JSON event
			var resizeEv struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if err := json.Unmarshal(msg, &resizeEv); err == nil && resizeEv.Type == "resize" {
				_ = agentSess.SendMessage(protocol.NewEnvelope(protocol.TypePTYResize, protocol.PTYResizePayload{
					SessionID: sessionID,
					Cols:      resizeEv.Cols,
					Rows:      resizeEv.Rows,
				}))
				continue
			}

			// Normal input keystrokes
			_ = agentSess.SendMessage(protocol.NewEnvelope(protocol.TypePTYData, protocol.PTYDataPayload{
				SessionID: sessionID,
				Data:      string(msg),
			}))
		} else if msgType == websocket.BinaryMessage {
			_ = agentSess.SendMessage(protocol.NewEnvelope(protocol.TypePTYData, protocol.PTYDataPayload{
				SessionID: sessionID,
				Data:      string(msg),
			}))
		}
	}

	// Notify agent to close PTY process
	_ = agentSess.SendMessage(protocol.NewEnvelope(protocol.TypePTYStop, protocol.PTYStopPayload{
		SessionID: sessionID,
	}))
}

func (h *Hub) closeTerminalSession(sessionID string) {
	h.termMu.Lock()
	defer h.termMu.Unlock()

	if term, ok := h.terminalSessions[sessionID]; ok {
		term.writeMu.Lock()
		term.closed = true
		_ = term.BrowserConn.Close()
		term.writeMu.Unlock()
		delete(h.terminalSessions, sessionID)
		slog.Info("terminal session closed", "session_id", sessionID)
	}
}

// GetAgent returns an active agent session
func (h *Hub) GetAgent(deviceID string) (*AgentSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	sess, ok := h.agents[deviceID]
	return sess, ok
}

// ConnectedCount returns count of currently connected agents
func (h *Hub) ConnectedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.agents)
}

// RequestRPC sends an RPC request envelope to the specified agent and awaits the response
func (h *Hub) RequestRPC(ctx context.Context, deviceID string, msgType string, payload interface{}) (protocol.Envelope, error) {
	h.mu.RLock()
	agent, ok := h.agents[deviceID]
	h.mu.RUnlock()
	if !ok || agent == nil {
		return protocol.Envelope{}, fmt.Errorf("device is not currently online")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("marshal payload error: %w", err)
	}

	var header struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(raw, &header); err != nil || header.RequestID == "" {
		return protocol.Envelope{}, fmt.Errorf("missing request_id in rpc payload")
	}

	ch := make(chan protocol.Envelope, 1)
	h.respMu.Lock()
	h.pendingResponses[header.RequestID] = ch
	h.respMu.Unlock()

	defer func() {
		h.respMu.Lock()
		delete(h.pendingResponses, header.RequestID)
		h.respMu.Unlock()
	}()

	env := protocol.NewEnvelope(msgType, payload)
	if err := agent.SendMessage(env); err != nil {
		return protocol.Envelope{}, fmt.Errorf("failed sending rpc request: %w", err)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return protocol.Envelope{}, ctx.Err()
	case <-time.After(10 * time.Second):
		return protocol.Envelope{}, fmt.Errorf("rpc request timed out")
	}
}

// ListFiles requests a directory listing from a remote agent
func (h *Hub) ListFiles(ctx context.Context, deviceID, targetPath string) (*protocol.FileListRespPayload, error) {
	reqID := uuid.NewString()
	req := protocol.FileListReqPayload{
		RequestID: reqID,
		Path:      targetPath,
	}

	env, err := h.RequestRPC(ctx, deviceID, protocol.TypeFileListReq, req)
	if err != nil {
		return nil, err
	}

	raw, _ := json.Marshal(env.Payload)
	var resp protocol.FileListRespPayload
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("invalid response payload: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return &resp, nil
}

// DownloadFileChunk requests a chunk of a remote file
func (h *Hub) DownloadFileChunk(ctx context.Context, deviceID, filePath string, offset int64, chunkSize int) (*protocol.FileDownloadChunkPayload, error) {
	reqID := uuid.NewString()
	req := protocol.FileDownloadReqPayload{
		RequestID: reqID,
		Path:      filePath,
		Offset:    offset,
		ChunkSize: chunkSize,
	}

	env, err := h.RequestRPC(ctx, deviceID, protocol.TypeFileDownloadReq, req)
	if err != nil {
		return nil, err
	}

	raw, _ := json.Marshal(env.Payload)
	var resp protocol.FileDownloadChunkPayload
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("invalid response payload: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return &resp, nil
}

// UploadFileChunk sends a file chunk to be written on the agent
func (h *Hub) UploadFileChunk(ctx context.Context, deviceID, filePath string, offset int64, b64Data string, isLast bool) error {
	reqID := uuid.NewString()
	req := protocol.FileUploadChunkPayload{
		RequestID: reqID,
		Path:      filePath,
		Offset:    offset,
		Data:      b64Data,
		IsLast:    isLast,
	}

	env, err := h.RequestRPC(ctx, deviceID, protocol.TypeFileUploadChunk, req)
	if err != nil {
		return err
	}

	raw, _ := json.Marshal(env.Payload)
	var resp protocol.FileUploadRespPayload
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("invalid response payload: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("agent error: %s", resp.Error)
	}
	return nil
}

// FileAction performs delete or mkdir on target agent
func (h *Hub) FileAction(ctx context.Context, deviceID, action, targetPath string) error {
	reqID := uuid.NewString()
	req := protocol.FileActionReqPayload{
		RequestID: reqID,
		Action:    action,
		Path:      targetPath,
	}

	env, err := h.RequestRPC(ctx, deviceID, protocol.TypeFileActionReq, req)
	if err != nil {
		return err
	}

	raw, _ := json.Marshal(env.Payload)
	var resp protocol.FileActionRespPayload
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("invalid response payload: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("agent error: %s", resp.Error)
	}
	return nil
}

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

// Hub manages active agent and browser sessions
type Hub struct {
	store  *store.Store
	agents map[string]*AgentSession // keyed by DeviceID
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates and starts a new Hub
func New(st *store.Store) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		store:  st,
		agents: make(map[string]*AgentSession),
		ctx:    ctx,
		cancel: cancel,
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

		default:
			slog.Debug("received message from agent", "type", env.Type, "device_id", dev.ID)
		}
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

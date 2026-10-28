package protocol

import "time"

// Message types for WebSocket communication
const (
	TypeHello        = "hello"
	TypeHelloAck     = "hello_ack"
	TypeHeartbeat    = "heartbeat"
	TypeHeartbeatAck = "heartbeat_ack"
	TypeTelemetry    = "telemetry"
	TypeError        = "error"
)

// Envelope wraps all messages exchanged over WebSocket
type Envelope struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

// HelloPayload is sent by the agent immediately after WebSocket handshake
type HelloPayload struct {
	DeviceID     string `json:"device_id"`
	HardwareUUID string `json:"hardware_uuid"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	InternalIP   string `json:"internal_ip"`
	AgentVersion string `json:"agent_version"`
	Token        string `json:"token,omitempty"`
}

// HelloAckPayload is returned by the server confirming successful registration
type HelloAckPayload struct {
	Accepted   bool   `json:"accepted"`
	Message    string `json:"message,omitempty"`
	ServerTime int64  `json:"server_time"`
}

// HeartbeatPayload sent periodically by agent
type HeartbeatPayload struct {
	DeviceID string  `json:"device_id"`
	CPUUsage float64 `json:"cpu_usage"`
	RAMUsage float64 `json:"ram_usage"`
	Uptime   uint64  `json:"uptime"`
}

// HeartbeatAckPayload acknowledged by server
type HeartbeatAckPayload struct {
	ReceivedAt int64 `json:"received_at"`
}

// TelemetryPayload carries detailed resource information
type TelemetryPayload struct {
	DeviceID    string  `json:"device_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	RAMUsedMB   uint64  `json:"ram_used_mb"`
	RAMTotalMB  uint64  `json:"ram_total_mb"`
	DiskUsedGB  uint64  `json:"disk_used_gb"`
	DiskTotalGB uint64  `json:"disk_total_gb"`
}

// NewEnvelope creates a new envelope with the current unix millisecond timestamp
func NewEnvelope(msgType string, payload interface{}) Envelope {
	return Envelope{
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

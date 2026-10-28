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

	// PTY Interactive Terminal message types
	TypePTYStart  = "pty_start"
	TypePTYData   = "pty_data"
	TypePTYResize = "pty_resize"
	TypePTYStop   = "pty_stop"

	// File Explorer & Transfer message types
	TypeFileListReq       = "file_list_req"
	TypeFileListResp      = "file_list_resp"
	TypeFileDownloadReq   = "file_download_req"
	TypeFileDownloadChunk = "file_download_chunk"
	TypeFileUploadChunk   = "file_upload_chunk"
	TypeFileUploadResp    = "file_upload_resp"
	TypeFileActionReq     = "file_action_req"
	TypeFileActionResp    = "file_action_resp"

	// Desktop Live Screen & Input message types
	TypeDesktopStart = "desktop_start"
	TypeDesktopStop  = "desktop_stop"
	TypeDesktopFrame = "desktop_frame"
	TypeDesktopInput = "desktop_input"
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
	GroupName    string `json:"group_name,omitempty"`
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

// PTYStartPayload tells the agent to spawn a PTY session
type PTYStartPayload struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

// PTYDataPayload carries terminal raw data chunks
type PTYDataPayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"` // string/byte content
}

// PTYResizePayload informs the agent of a window size change
type PTYResizePayload struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

// PTYStopPayload requests termination of a PTY session
type PTYStopPayload struct {
	SessionID string `json:"session_id"`
}

// FileEntry represents a file or directory item
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// FileListReqPayload requests a directory listing
type FileListReqPayload struct {
	RequestID string `json:"request_id"`
	Path      string `json:"path"`
}

// FileListRespPayload returns directory items
type FileListRespPayload struct {
	RequestID string      `json:"request_id"`
	Path      string      `json:"path"`
	Entries   []FileEntry `json:"entries"`
	Error     string      `json:"error,omitempty"`
}

// FileDownloadReqPayload requests a file to download
type FileDownloadReqPayload struct {
	RequestID string `json:"request_id"`
	Path      string `json:"path"`
	Offset    int64  `json:"offset"`
	ChunkSize int    `json:"chunk_size"`
}

// FileDownloadChunkPayload returns a chunk of file data
type FileDownloadChunkPayload struct {
	RequestID string `json:"request_id"`
	Offset    int64  `json:"offset"`
	Data      string `json:"data"` // base64 encoded
	IsEOF     bool   `json:"is_eof"`
	Error     string `json:"error,omitempty"`
}

// FileUploadChunkPayload delivers a chunk of data to write on agent
type FileUploadChunkPayload struct {
	RequestID string `json:"request_id"`
	Path      string `json:"path"`
	Offset    int64  `json:"offset"`
	Data      string `json:"data"` // base64 encoded
	IsLast    bool   `json:"is_last"`
}

// FileUploadRespPayload confirms a chunk write
type FileUploadRespPayload struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// FileActionReqPayload requests file operations (delete, mkdir)
type FileActionReqPayload struct {
	RequestID string `json:"request_id"`
	Action    string `json:"action"` // "delete", "mkdir"
	Path      string `json:"path"`
}

// FileActionRespPayload confirms action outcome
type FileActionRespPayload struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// DesktopStartPayload requests starting a screen capture stream
type DesktopStartPayload struct {
	SessionID string `json:"session_id"`
	Display   int    `json:"display"` // default 0
	Quality   int    `json:"quality"` // 1 - 100 (default 65)
	FPS       int    `json:"fps"`     // default 15
}

// DesktopStopPayload requests stopping a screen capture stream
type DesktopStopPayload struct {
	SessionID string `json:"session_id"`
}

// DesktopFramePayload carries a captured video/image frame
type DesktopFramePayload struct {
	SessionID string `json:"session_id"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"format"` // "jpeg"
	Data      string `json:"data"`   // base64 encoded
}

// DesktopInputPayload carries mouse/keyboard interactions
type DesktopInputPayload struct {
	SessionID string  `json:"session_id"`
	Event     string  `json:"event"` // "mousemove", "mousedown", "mouseup", "click", "keydown", "keyup", "wheel"
	X         float64 `json:"x"`     // normalized 0.0 - 1.0 or pixel coordinate
	Y         float64 `json:"y"`
	Button    int     `json:"button"` // 0=left, 1=middle, 2=right
	Key       string  `json:"key,omitempty"`
	DeltaY    float64 `json:"delta_y,omitempty"`
}

// NewEnvelope creates a new envelope with the current unix millisecond timestamp
func NewEnvelope(msgType string, payload interface{}) Envelope {
	return Envelope{
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

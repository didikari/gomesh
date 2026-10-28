package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gomesh/internal/agent"
	"gomesh/internal/protocol"
	"gomesh/internal/server/store"

	"github.com/gorilla/websocket"
)

func TestRemoteDesktopStream(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_hub.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer st.Close()

	h := New(st)
	defer h.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/agent", func(w http.ResponseWriter, r *http.Request) {
		h.HandleAgentWS(w, r)
	})
	mux.HandleFunc("GET /ws/desktop", func(w http.ResponseWriter, r *http.Request) {
		h.HandleDesktopWS(w, r)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agent"

	// Start agent in background
	ag, err := agent.NewClient(wsURL, tempDir)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ag.Run(ctx)

	// Wait for agent to become online
	var devID string
	for i := 0; i < 20; i++ {
		devices, err := st.ListDevices(context.Background())
		if err == nil && len(devices) > 0 && devices[0].Status == "online" {
			devID = devices[0].ID
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if devID == "" {
		t.Fatalf("agent did not become online")
	}

	// Connect browser desktop WebSocket client
	deskWSURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/desktop?device_id=" + devID
	browserConn, _, err := websocket.DefaultDialer.Dial(deskWSURL, nil)
	if err != nil {
		t.Fatalf("failed connecting desktop ws: %v", err)
	}
	defer browserConn.Close()

	// Wait for at least 1 desktop frame
	browserConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var frame protocol.DesktopFramePayload
	err = browserConn.ReadJSON(&frame)
	if err != nil {
		t.Fatalf("failed reading desktop frame: %v", err)
	}

	if frame.Format != "jpeg" {
		t.Errorf("expected frame format jpeg, got %s", frame.Format)
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		t.Errorf("expected valid resolution, got %dx%d", frame.Width, frame.Height)
	}
	if frame.Data == "" {
		t.Errorf("expected non-empty frame data")
	}

	// Send an input event
	input := protocol.DesktopInputPayload{
		Event: "mousemove",
		X:     100,
		Y:     100,
	}
	if err := browserConn.WriteJSON(input); err != nil {
		t.Fatalf("failed sending desktop input: %v", err)
	}
}

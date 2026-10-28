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
	"gomesh/internal/server/store"

	"github.com/gorilla/websocket"
)

func TestTerminalPTYRelay(t *testing.T) {
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
	mux.HandleFunc("GET /ws/terminal", func(w http.ResponseWriter, r *http.Request) {
		h.HandleTerminalWS(w, r)
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

	// Wait for agent to register
	time.Sleep(100 * time.Millisecond)
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

	// Connect browser terminal WebSocket client
	termWSURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/terminal?device_id=" + devID
	browserConn, _, err := websocket.DefaultDialer.Dial(termWSURL, nil)
	if err != nil {
		t.Fatalf("failed to connect terminal ws: %v", err)
	}
	defer browserConn.Close()

	// Read initial welcome prompt first to make sure PTY is ready
	time.Sleep(300 * time.Millisecond)

	// Send shell command
	testToken := "P_T_Y_TEST_OK"
	cmd := "echo " + testToken + "\n"
	if err := browserConn.WriteMessage(websocket.TextMessage, []byte(cmd)); err != nil {
		t.Fatalf("failed writing command to terminal ws: %v", err)
	}

	// Read output and search for testToken
	found := false
	browserConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := browserConn.ReadMessage()
		if err != nil {
			t.Logf("read error: %v", err)
			break
		}
		t.Logf("received from terminal: %q", string(msg))
		if strings.Contains(string(msg), testToken) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected output to contain token %s", testToken)
	}
}

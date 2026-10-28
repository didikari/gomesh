package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gomesh/internal/protocol"

	"github.com/gorilla/websocket"
)

func TestClientEndpointsConfiguration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomesh-agent-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	client, err := NewClient("ws://localhost:8080/ws/agent", tempDir)
	if err != nil {
		t.Fatalf("failed creating client: %v", err)
	}

	eps := client.GetEndpoints()
	if len(eps) != 1 || eps[0] != "ws://localhost:8080/ws/agent" {
		t.Fatalf("unexpected default endpoints: %v", eps)
	}

	// Configure multiple endpoints (LAN first, then Public)
	client.SetEndpoints([]string{
		"ws://192.168.1.5:8080/ws/agent",
		"wss://mesh.trycloudflare.com/ws/agent",
	})

	eps = client.GetEndpoints()
	if len(eps) != 3 { // 2 explicit + 1 fallback server_url
		t.Fatalf("expected 3 endpoints, got %d: %v", len(eps), eps)
	}
	if eps[0] != "ws://192.168.1.5:8080/ws/agent" {
		t.Errorf("expected first endpoint to be LAN: %s", eps[0])
	}
	if eps[1] != "wss://mesh.trycloudflare.com/ws/agent" {
		t.Errorf("expected second endpoint to be Public: %s", eps[1])
	}
}

func TestClientEndpointFailover(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomesh-agent-failover-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock server for Candidate 2 (Fallback)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	fallbackHit := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/agent" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			var env protocol.Envelope
			if err := conn.ReadJSON(&env); err == nil && env.Type == protocol.TypeHello {
				_ = conn.WriteJSON(protocol.NewEnvelope(protocol.TypeHelloAck, protocol.HelloAckPayload{
					Accepted: true,
					Message:  "Connected to fallback server",
				}))
				fallbackHit <- struct{}{}
			}

			// Keep connection open until closed by client
			for {
				var dummy protocol.Envelope
				if err := conn.ReadJSON(&dummy); err != nil {
					break
				}
			}
		}
	}))
	defer server.Close()

	fallbackWS := "ws://" + server.Listener.Addr().String() + "/ws/agent"
	unreachableWS := "ws://127.0.0.1:59999/ws/agent" // Non-existent port

	client, err := NewClient(unreachableWS, tempDir)
	if err != nil {
		t.Fatalf("failed creating client: %v", err)
	}

	// Endpoints: unreachable LAN first, then reachable fallback
	client.SetEndpoints([]string{
		unreachableWS,
		fallbackWS,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- client.Run(ctx)
	}()

	select {
	case <-fallbackHit:
		// Succeeded in falling over to candidate 2!
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timed out waiting for agent to failover to candidate 2")
	}

	<-errChan
}

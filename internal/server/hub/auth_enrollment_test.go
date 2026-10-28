package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gomesh/internal/agent"
	"gomesh/internal/server/auth"
	"gomesh/internal/server/store"

	"github.com/gorilla/websocket"
)

func TestAuthAndTokenEnrollment(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomesh-auth-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// Create admin user
	_, _, err = st.EnsureDefaultAdmin(context.Background())
	if err != nil {
		t.Fatalf("failed creating default admin: %v", err)
	}

	// Create a valid enrollment token
	validToken, err := st.CreateEnrollmentToken(context.Background(), "Production Enrollment Token", "admin", 7)
	if err != nil {
		t.Fatalf("failed creating token: %v", err)
	}

	h := New(st)
	defer h.Stop()

	sm := auth.NewSessionManager(time.Hour)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/agent", func(w http.ResponseWriter, r *http.Request) {
		h.HandleAgentWS(w, r)
	})
	mux.HandleFunc("GET /ws/terminal", sm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.HandleTerminalWS(w, r)
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agent"

	// 1. Test Agent with INVALID token -> Must be rejected
	t.Run("AgentWithInvalidTokenRejected", func(t *testing.T) {
		invalidAgentDir := filepath.Join(tempDir, "invalid-agent")
		_ = os.MkdirAll(invalidAgentDir, 0755)

		agInvalid, err := agent.NewClient(wsURL, invalidAgentDir)
		if err != nil {
			t.Fatalf("failed to create agent: %v", err)
		}
		agInvalid.SetAuthToken("totally-invalid-token-12345")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		go agInvalid.Run(ctx)

		time.Sleep(300 * time.Millisecond)

		// Check devices in store -> should NOT be registered or online
		devs, err := st.ListDevices(context.Background())
		if err != nil {
			t.Fatalf("error listing devices: %v", err)
		}
		for _, d := range devs {
			if d.Status == "online" {
				t.Fatalf("expected no device online with invalid token, found %s", d.Hostname)
			}
		}
	})

	// 2. Test Agent with VALID token -> Must connect successfully
	var enrolledDeviceID string
	t.Run("AgentWithValidTokenEnrolled", func(t *testing.T) {
		validAgentDir := filepath.Join(tempDir, "valid-agent")
		_ = os.MkdirAll(validAgentDir, 0755)

		agValid, err := agent.NewClient(wsURL, validAgentDir)
		if err != nil {
			t.Fatalf("failed to create agent: %v", err)
		}
		agValid.SetAuthToken(validToken.Token)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go agValid.Run(ctx)

		// Wait for agent to become online
		var targetDev *store.Device
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			devices, _ := st.ListDevices(context.Background())
			for _, d := range devices {
				if d.Status == "online" {
					targetDev = d
					break
				}
			}
			if targetDev != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if targetDev == nil {
			t.Fatal("agent with valid token failed to register as online")
		}

		enrolledDeviceID = targetDev.ID

		// 3. Test Session Audit Logging
		t.Run("SessionAuditLogged", func(t *testing.T) {
			// Authenticate user session
			user, err := st.AuthenticateUser(context.Background(), "admin", "admin123")
			if err != nil {
				t.Fatalf("failed authenticating user: %v", err)
			}
			sess := sm.Create(user)

			termWSURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/terminal?device_id=" + enrolledDeviceID
			header := http.Header{}
			header.Add("Cookie", auth.SessionCookieName+"="+sess.ID)

			conn, _, err := websocket.DefaultDialer.Dial(termWSURL, header)
			if err != nil {
				t.Fatalf("failed connecting terminal WS: %v", err)
			}

			// Read or write brief message then close
			time.Sleep(150 * time.Millisecond)
			conn.Close()

			// Wait for audit record to be flushed
			time.Sleep(150 * time.Millisecond)

			audits, err := st.ListAudits(context.Background(), 10)
			if err != nil {
				t.Fatalf("failed listing audits: %v", err)
			}

			if len(audits) == 0 {
				t.Fatal("expected at least 1 session audit log, got 0")
			}

			latest := audits[0]
			if latest.Username != "admin" || latest.SessionType != "terminal" || latest.DeviceID != enrolledDeviceID {
				t.Fatalf("unexpected audit entry: %+v", latest)
			}
		})
	})
}

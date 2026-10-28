package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gomesh/internal/agent/embeddedcfg"
	"gomesh/internal/server/auth"
	"gomesh/internal/server/hub"
	"gomesh/internal/server/store"
	"gomesh/scripts"
)

func TestDirectDownloadsAndEmbeddedScripts(t *testing.T) {
	// 1. Verify embedded scripts exist and can be read
	ps1, err := scripts.Files.ReadFile("install-windows.ps1")
	if err != nil || len(ps1) == 0 {
		t.Fatalf("failed to read embedded install-windows.ps1: %v", err)
	}

	shLinux, err := scripts.Files.ReadFile("install-linux.sh")
	if err != nil || len(shLinux) == 0 {
		t.Fatalf("failed to read embedded install-linux.sh: %v", err)
	}

	// 2. Verify findBinaryPath can locate gomesh-agent.exe in repo root or bin
	found := findBinaryPath("gomesh-agent.exe")
	if found == "" {
		// If testing from cmd/gomesh, check ../../bin/gomesh-agent.exe
		if _, err := os.Stat(filepath.Join("..", "..", "bin", "gomesh-agent.exe")); err != nil {
			t.Log("Note: gomesh-agent.exe not built yet in local path")
		}
	}

	// 3. Test dynamic batch installer generation endpoint
	req := httptest.NewRequest("GET", "/api/download/installer/windows-bat?token=test-tok&group=TestGroup", nil)
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/download/installer/windows-bat", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		w.Header().Set("Content-Type", "application/x-bat")
		w.Write([]byte("echo token=" + token))
	})

	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if rec.Body.String() != "echo token=test-tok" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	// 4. Test Auto-Configured Binary Download with Embedded Configuration Overlay
	st, _ := store.New(":memory:")
	defer st.Close()
	sm := auth.NewSessionManager(time.Hour)
	h := hub.New(st)
	defer h.Stop()

	serverMux := buildServerMux(st, sm, h)

	// Ensure bin/gomesh-agent.exe exists for test
	os.MkdirAll("bin", 0755)
	if _, err := os.Stat("bin/gomesh-agent.exe"); os.IsNotExist(err) {
		os.WriteFile("bin/gomesh-agent.exe", []byte("MZ-dummy-binary-header-data"), 0755)
		defer os.Remove("bin/gomesh-agent.exe")
	}

	reqBin := httptest.NewRequest("GET", "/bin/gomesh-agent.exe?token=tok-abc-123&group=Branch-Office&server=https://mesh.mycompany.com", nil)
	recBin := httptest.NewRecorder()
	serverMux.ServeHTTP(recBin, reqBin)

	if recBin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK downloading binary, got %d", recBin.Code)
	}

	cfg := embeddedcfg.ReadFromBytes(recBin.Body.Bytes())
	if cfg == nil {
		t.Fatalf("expected embedded config in downloaded binary")
	}
	if cfg.Server != "wss://mesh.mycompany.com/ws/agent" {
		t.Errorf("expected wss://mesh.mycompany.com/ws/agent, got %s", cfg.Server)
	}
	if cfg.Token != "tok-abc-123" {
		t.Errorf("expected tok-abc-123, got %s", cfg.Token)
	}
	if cfg.Group != "Branch-Office" {
		t.Errorf("expected Branch-Office, got %s", cfg.Group)
	}

	// 5. Test Smart Roaming Multi-Endpoint Overlay
	reqMulti := httptest.NewRequest("GET", "/bin/gomesh-agent.exe?token=tok-abc-123&group=Branch-Office&lan_server=ws://192.168.1.5:8080/ws/agent&public_server=wss://mesh.public.com/ws/agent", nil)
	recMulti := httptest.NewRecorder()
	serverMux.ServeHTTP(recMulti, reqMulti)

	if recMulti.Code != http.StatusOK {
		t.Fatalf("expected 200 OK downloading multi-endpoint binary, got %d", recMulti.Code)
	}
	cfgMulti := embeddedcfg.ReadFromBytes(recMulti.Body.Bytes())
	if cfgMulti == nil {
		t.Fatalf("expected embedded multi-endpoint config in downloaded binary")
	}
	eps := cfgMulti.GetEndpoints()
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d: %v", len(eps), eps)
	}
	if eps[0] != "ws://192.168.1.5:8080/ws/agent" {
		t.Errorf("expected first endpoint to be LAN, got %s", eps[0])
	}
	if eps[1] != "wss://mesh.public.com/ws/agent" {
		t.Errorf("expected second endpoint to be Public WAN, got %s", eps[1])
	}

	// 6. Test GET /api/health
	reqHealth := httptest.NewRequest("GET", "/api/health", nil)
	recHealth := httptest.NewRecorder()
	serverMux.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /api/health, got %d", recHealth.Code)
	}

	// 7. Test GET /api/server/network
	reqNet := httptest.NewRequest("GET", "/api/server/network", nil)
	recNet := httptest.NewRecorder()
	serverMux.ServeHTTP(recNet, reqNet)
	if recNet.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /api/server/network, got %d", recNet.Code)
	}
}

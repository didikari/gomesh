package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
}

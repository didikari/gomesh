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
	"gomesh/internal/server/store"
)

func TestFileExplorerRPC(t *testing.T) {
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

	// 1. Test ListFiles
	listResp, err := h.ListFiles(context.Background(), devID, tempDir)
	if err != nil {
		t.Fatalf("failed listing files: %v", err)
	}
	if listResp == nil {
		t.Fatalf("expected list response, got nil")
	}

	// 2. Test File Action (mkdir)
	newFolderPath := filepath.Join(tempDir, "test_folder_mesh")
	if err := h.FileAction(context.Background(), devID, "mkdir", newFolderPath); err != nil {
		t.Fatalf("failed creating folder: %v", err)
	}
	if _, err := os.Stat(newFolderPath); err != nil {
		t.Fatalf("folder was not created on disk: %v", err)
	}

	// 3. Test File Action (delete)
	if err := h.FileAction(context.Background(), devID, "delete", newFolderPath); err != nil {
		t.Fatalf("failed deleting folder: %v", err)
	}
	if _, err := os.Stat(newFolderPath); !os.IsNotExist(err) {
		t.Fatalf("folder should have been deleted")
	}
}

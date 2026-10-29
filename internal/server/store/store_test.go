package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreUpsertAndHeartbeat(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_gomesh.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	dev := &Device{
		ID:           "test-dev-1",
		HardwareUUID: "uuid-12345",
		Hostname:     "server-node-1",
		OS:           "linux",
		Arch:         "amd64",
		InternalIP:   "10.0.0.5",
		PublicIP:     "203.0.113.195",
		AgentVersion: "0.1.0",
		Status:       "online",
	}

	if err := s.UpsertDevice(ctx, dev); err != nil {
		t.Fatalf("failed to upsert device: %v", err)
	}

	fetched, err := s.GetDevice(ctx, dev.ID)
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected device, got nil")
	}
	if fetched.Hostname != "server-node-1" {
		t.Errorf("expected hostname server-node-1, got %s", fetched.Hostname)
	}

	// Update heartbeat
	if err := s.UpdateHeartbeat(ctx, dev.ID, 12.5, 45.2); err != nil {
		t.Fatalf("failed to update heartbeat: %v", err)
	}

	fetched, _ = s.GetDevice(ctx, dev.ID)
	if fetched.CPUUsage != 12.5 {
		t.Errorf("expected CPU 12.5, got %f", fetched.CPUUsage)
	}

	// Mark stale devices
	affected, err := s.MarkStaleDevices(ctx, 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("failed to mark stale: %v", err)
	}
	if affected == 0 {
		t.Errorf("expected at least 1 affected device, got %d", affected)
	}

	fetched, _ = s.GetDevice(ctx, dev.ID)
	if fetched.Status != "offline" {
		t.Errorf("expected status offline, got %s", fetched.Status)
	}

	// Delete device
	if err := s.DeleteDevice(ctx, dev.ID); err != nil {
		t.Fatalf("failed to delete device: %v", err)
	}

	fetched, err = s.GetDevice(ctx, dev.ID)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if fetched != nil {
		t.Fatalf("expected device to be deleted, but still found: %v", fetched)
	}

	// Deleting non-existent device should return error
	if err := s.DeleteDevice(ctx, "non-existent-id"); err == nil {
		t.Errorf("expected error when deleting non-existent device, got nil")
	}
}

package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDeviceGrouping(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomesh-group-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "group.db")
	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed creating store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// 1. Insert device in "Group A"
	devA := &Device{
		ID:           uuid.NewString(),
		HardwareUUID: "uuid-device-a",
		Hostname:     "server-alpha",
		OS:           "linux",
		Arch:         "amd64",
		GroupName:    "Group A",
		Status:       "online",
		LastSeenAt:   time.Now().UTC(),
	}
	if err := st.UpsertDevice(ctx, devA); err != nil {
		t.Fatalf("failed upserting devA: %v", err)
	}

	// 2. Insert device in "Group B"
	devB := &Device{
		ID:           uuid.NewString(),
		HardwareUUID: "uuid-device-b",
		Hostname:     "laptop-beta",
		OS:           "windows",
		Arch:         "amd64",
		GroupName:    "Group B",
		Status:       "online",
		LastSeenAt:   time.Now().UTC(),
	}
	if err := st.UpsertDevice(ctx, devB); err != nil {
		t.Fatalf("failed upserting devB: %v", err)
	}

	// 3. List groups -> should return ["Group A", "Group B"]
	groups, err := st.ListDeviceGroups(ctx)
	if err != nil {
		t.Fatalf("failed listing groups: %v", err)
	}
	if len(groups) != 2 || groups[0] != "Group A" || groups[1] != "Group B" {
		t.Fatalf("expected ['Group A', 'Group B'], got %v", groups)
	}

	// 4. Update device B to "Group A"
	if err := st.UpdateDeviceGroup(ctx, devB.ID, "Group A"); err != nil {
		t.Fatalf("failed updating group: %v", err)
	}

	// 5. Verify device B is now in "Group A"
	updatedB, err := st.GetDevice(ctx, devB.ID)
	if err != nil {
		t.Fatalf("failed getting devB: %v", err)
	}
	if updatedB.GroupName != "Group A" {
		t.Fatalf("expected devB to be in 'Group A', got '%s'", updatedB.GroupName)
	}

	// 6. Groups list should now only be ["Group A"]
	groupsAfter, err := st.ListDeviceGroups(ctx)
	if err != nil {
		t.Fatalf("failed listing groups after update: %v", err)
	}
	if len(groupsAfter) != 1 || groupsAfter[0] != "Group A" {
		t.Fatalf("expected ['Group A'], got %v", groupsAfter)
	}
}

func TestLegacyDatabaseMigrationWithoutGroupName(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomesh-legacy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "legacy.db")

	// Create legacy database with old schema (without group_name column)
	rawDB, err := openRawDB(dbPath)
	if err != nil {
		t.Fatalf("failed creating raw db: %v", err)
	}
	legacySchema := `
	CREATE TABLE devices (
		id TEXT PRIMARY KEY,
		hardware_uuid TEXT UNIQUE NOT NULL,
		hostname TEXT NOT NULL,
		os TEXT NOT NULL,
		arch TEXT NOT NULL,
		internal_ip TEXT,
		public_ip TEXT,
		agent_version TEXT,
		status TEXT NOT NULL DEFAULT 'offline',
		cpu_usage REAL DEFAULT 0.0,
		ram_usage REAL DEFAULT 0.0,
		last_seen_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);
	INSERT INTO devices (id, hardware_uuid, hostname, os, arch, status, last_seen_at, created_at)
	VALUES ('old-1', 'hw-uuid-1', 'legacy-pc', 'windows', 'amd64', 'online', '2026-10-28 10:00:00', '2026-10-28 10:00:00');
	`
	if _, err := rawDB.Exec(legacySchema); err != nil {
		rawDB.Close()
		t.Fatalf("failed creating legacy table: %v", err)
	}
	rawDB.Close()

	// Now open using store.New - it must perform migration without error
	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed migrating legacy db: %v", err)
	}
	defer st.Close()

	devs, err := st.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("failed listing devices after legacy migration: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("expected 1 legacy device, got %d", len(devs))
	}
	if devs[0].GroupName != "Default" {
		t.Fatalf("expected default group for legacy device, got '%s'", devs[0].GroupName)
	}
}

func openRawDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

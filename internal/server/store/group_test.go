package store

import (
	"context"
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

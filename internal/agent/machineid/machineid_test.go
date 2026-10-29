package machineid

import (
	"testing"
)

func TestGet(t *testing.T) {
	id1 := Get()
	if id1 == "" {
		t.Fatalf("expected non-empty machine ID, got empty")
	}

	id2 := Get()
	if id1 != id2 {
		t.Fatalf("expected deterministic machine ID, got %s vs %s", id1, id2)
	}
	t.Logf("Detected Machine ID: %s", id1)
}

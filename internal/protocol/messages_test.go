package protocol

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeSerialization(t *testing.T) {
	hello := HelloPayload{
		DeviceID:     "dev-123",
		HardwareUUID: "uuid-abc",
		Hostname:     "macbook-pro",
		OS:           "darwin",
		Arch:         "arm64",
		InternalIP:   "192.168.1.10",
		AgentVersion: "0.1.0",
	}

	env := NewEnvelope(TypeHello, hello)
	bytes, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}

	var decoded Envelope
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}

	if decoded.Type != TypeHello {
		t.Errorf("expected type %s, got %s", TypeHello, decoded.Type)
	}
	if decoded.Timestamp <= 0 {
		t.Errorf("expected positive timestamp, got %d", decoded.Timestamp)
	}
}

package embeddedcfg

import (
	"bytes"
	"testing"
)

func TestOverlayConfigAppendAndRead(t *testing.T) {
	dummyExe := []byte("MZ\x90\x00\x03\x00\x00\x00...this-is-a-compiled-binary-payload-1234567890...")

	cfg := AgentConfig{
		Server:    "wss://mycentral.cloud:8080/ws/agent",
		Endpoints: []string{"ws://192.168.1.5:8080/ws/agent", "wss://mycentral.cloud:8080/ws/agent"},
		Token:     "test-secret-enrollment-token",
		Group:     "Production-Servers",
	}

	var output bytes.Buffer
	err := AppendConfigToBinary(bytes.NewReader(dummyExe), &output, cfg)
	if err != nil {
		t.Fatalf("failed appending config: %v", err)
	}

	read := ReadFromBytes(output.Bytes())
	if read == nil {
		t.Fatalf("failed reading config from appended binary bytes")
	}

	if read.Server != cfg.Server {
		t.Errorf("expected server %s, got %s", cfg.Server, read.Server)
	}
	if read.Token != cfg.Token {
		t.Errorf("expected token %s, got %s", cfg.Token, read.Token)
	}
	if read.Group != cfg.Group {
		t.Errorf("expected group %s, got %s", cfg.Group, read.Group)
	}

	eps := read.GetEndpoints()
	if len(eps) != 2 || eps[0] != "ws://192.168.1.5:8080/ws/agent" || eps[1] != "wss://mycentral.cloud:8080/ws/agent" {
		t.Errorf("unexpected endpoints: %v", eps)
	}
}

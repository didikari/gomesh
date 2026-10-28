package embeddedcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// TagStart marks the beginning of the embedded JSON configuration block
	TagStart = "\n<!--GOMESH_CFG_START-->\n"
	// TagEnd marks the end of the embedded JSON configuration block
	TagEnd = "\n<!--GOMESH_CFG_END-->\n"
)

// AgentConfig represents the configuration baked into an agent binary
type AgentConfig struct {
	Server    string   `json:"server,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
	Token     string   `json:"token,omitempty"`
	Group     string   `json:"group,omitempty"`
}

// GetEndpoints returns an ordered, deduplicated slice of server endpoints to attempt (e.g. LAN first, then Public)
func (c AgentConfig) GetEndpoints() []string {
	var list []string
	seen := make(map[string]bool)

	add := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			list = append(list, u)
		}
	}

	for _, ep := range c.Endpoints {
		add(ep)
	}
	if c.Server != "" {
		add(c.Server)
	}
	return list
}

// AppendConfigToBinary writes the original binary followed by the embedded configuration block
func AppendConfigToBinary(r io.Reader, w io.Writer, cfg AgentConfig) error {
	if _, err := io.Copy(w, r); err != nil {
		return fmt.Errorf("failed copying base binary: %w", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed marshaling config: %w", err)
	}

	block := fmt.Sprintf("%s%s%s", TagStart, string(data), TagEnd)
	if _, err := w.Write([]byte(block)); err != nil {
		return fmt.Errorf("failed writing config block: %w", err)
	}

	return nil
}

// ReadFromBytes attempts to parse the embedded config from the tail of a byte slice
func ReadFromBytes(b []byte) *AgentConfig {
	startIdx := bytes.LastIndex(b, []byte(TagStart))
	if startIdx == -1 {
		return nil
	}
	startIdx += len(TagStart)

	endIdx := bytes.Index(b[startIdx:], []byte(TagEnd))
	if endIdx == -1 {
		return nil
	}

	jsonBytes := b[startIdx : startIdx+endIdx]
	var cfg AgentConfig
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// ReadFromCurrentExecutable reads the embedded config from the running binary
func ReadFromCurrentExecutable() *AgentConfig {
	execPath, err := os.Executable()
	if err != nil {
		return nil
	}
	realPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realPath
	}

	f, err := os.Open(execPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.Size() < 512 {
		return nil
	}

	// Read up to last 64KB
	readSize := int64(65536)
	if fi.Size() < readSize {
		readSize = fi.Size()
	}
	buf := make([]byte, readSize)
	n, err := f.ReadAt(buf, fi.Size()-readSize)
	if err != nil && err != io.EOF {
		return nil
	}
	return ReadFromBytes(buf[:n])
}

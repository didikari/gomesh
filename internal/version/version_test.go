package version

import (
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	v := Get()
	if v == "" {
		t.Fatalf("expected non-empty version, got empty string")
	}
	if !strings.HasPrefix(v, "v") {
		t.Errorf("expected version to start with 'v', got %s", v)
	}

	info := Info()
	if info["version"] != v {
		t.Errorf("expected Info()['version'] == %s, got %s", v, info["version"])
	}
}

func TestFormatVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"0.1.0", "v0.1.0"},
		{"v0.1.7", "v0.1.7"},
		{"  v1.0.0  ", "v1.0.0"},
		{"1.2.3", "v1.2.3"},
		{"", ""},
	}

	for _, c := range cases {
		actual := formatVersion(c.input)
		if actual != c.expected {
			t.Errorf("formatVersion(%q) = %q, want %q", c.input, actual, c.expected)
		}
	}
}

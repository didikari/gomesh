//go:build linux

package machineid

import (
	"os"
	"strings"
)

func getPlatformID() (string, error) {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id, nil
			}
		}
	}
	return "", os.ErrNotExist
}

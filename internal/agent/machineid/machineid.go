package machineid

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/google/uuid"
)

// Get returns a persistent hardware machine identifier for the host operating system.
func Get() string {
	if id, err := getPlatformID(); err == nil && strings.TrimSpace(id) != "" {
		return strings.ToLower(strings.TrimSpace(id))
	}

	var macStr string
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 {
				macStr = iface.HardwareAddr.String()
				break
			}
		}
	}

	hostname, _ := os.Hostname()
	if macStr != "" || hostname != "" {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", macStr, hostname)))
		u, err := uuid.FromBytes(hash[:16])
		if err == nil {
			return u.String()
		}
	}

	return uuid.NewString()
}

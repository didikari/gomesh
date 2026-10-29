//go:build darwin

package machineid

import (
	"os/exec"
	"regexp"
	"strings"
)

func getPlatformID() (string, error) {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}
	return "", nil
}

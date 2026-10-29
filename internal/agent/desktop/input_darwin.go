//go:build darwin

package desktop

import (
	"fmt"
	"os/exec"
	"strings"

	"gomesh/internal/protocol"
)

func injectInput(payload protocol.DesktopInputPayload) error {
	// 1. If cliclick is installed, it provides sub-millisecond native Quartz input injection
	if cliclickPath, err := exec.LookPath("cliclick"); err == nil {
		switch payload.Event {
		case "mousemove":
			return exec.Command(cliclickPath, fmt.Sprintf("m:%d,%d", int(payload.X), int(payload.Y))).Run()
		case "click", "mousedown":
			cmd := "c"
			if payload.Button == 2 {
				cmd = "rc"
			}
			return exec.Command(cliclickPath, fmt.Sprintf("%s:%d,%d", cmd, int(payload.X), int(payload.Y))).Run()
		case "wheel":
			dist := 5
			if payload.DeltaY > 0 {
				dist = -5
			}
			return exec.Command(cliclickPath, fmt.Sprintf("m:%d,%d", int(payload.X), int(payload.Y)), fmt.Sprintf("w:%d", dist)).Run()
		case "keydown":
			if payload.Key != "" {
				return exec.Command(cliclickPath, fmt.Sprintf("t:%s", payload.Key)).Run()
			}
		}
		return nil
	}

	// 2. Fallback to built-in macOS osascript (standard on 100% of Macs)
	switch payload.Event {
	case "click", "mousedown":
		script := fmt.Sprintf(`tell application "System Events" to click at {%d, %d}`, int(payload.X), int(payload.Y))
		if payload.Button == 2 {
			script = fmt.Sprintf(`
				tell application "System Events"
					key down control
					click at {%d, %d}
					key up control
				end tell`, int(payload.X), int(payload.Y))
		}
		_ = exec.Command("osascript", "-e", script).Run()

	case "wheel":
		// osascript scroll simulation via key codes (PageUp / PageDown)
		keyCode := "121" // PageDown
		if payload.DeltaY < 0 {
			keyCode = "116" // PageUp
		}
		script := fmt.Sprintf(`tell application "System Events" to key code %s`, keyCode)
		_ = exec.Command("osascript", "-e", script).Run()

	case "keydown":
		if payload.Key != "" {
			escaped := strings.ReplaceAll(payload.Key, `"`, `\"`)
			script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped)
			_ = exec.Command("osascript", "-e", script).Run()
		}
	}

	return nil
}

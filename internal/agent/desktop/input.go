package desktop

import (
	"gomesh/internal/protocol"
)

// HandleInput dispatches mouse and keyboard events to the OS subsystem
func HandleInput(payload protocol.DesktopInputPayload) error {
	return injectInput(payload)
}

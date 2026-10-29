//go:build !windows && !linux && !darwin

package desktop

import (
	"log/slog"

	"gomesh/internal/protocol"
)

func injectInput(payload protocol.DesktopInputPayload) error {
	slog.Debug("input simulation not implemented for target OS", "event", payload.Event)
	return nil
}

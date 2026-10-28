//go:build !windows

package desktop

import (
	"log/slog"

	"gomesh/internal/protocol"
)

func injectInput(payload protocol.DesktopInputPayload) error {
	slog.Debug("simulating input event on unix target", "event", payload.Event, "x", payload.X, "y", payload.Y)
	return nil
}

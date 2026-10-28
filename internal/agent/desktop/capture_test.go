package desktop

import (
	"runtime"
	"testing"
)

func TestCaptureDisplay(t *testing.T) {
	frame, err := CaptureDisplay(0, 65)
	if err != nil {
		t.Logf("CaptureDisplay error on %s: %v", runtime.GOOS, err)
	} else {
		t.Logf("CaptureDisplay success: %dx%d, format=%s, dataLen=%d", frame.Width, frame.Height, frame.Format, len(frame.Data))
	}
}

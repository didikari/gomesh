package desktop

import (
	"testing"
)

func TestCaptureDisplay(t *testing.T) {
	frame65, _ := CaptureDisplay(0, 65)
	frame45, _ := CaptureDisplay(0, 45)
	frame35, _ := CaptureDisplay(0, 35)

	if frame65 != nil {
		t.Logf("Quality 65 dataLen=%d", len(frame65.Data))
	}
	if frame45 != nil {
		t.Logf("Quality 45 dataLen=%d", len(frame45.Data))
	}
	if frame35 != nil {
		t.Logf("Quality 35 dataLen=%d", len(frame35.Data))
	}
}

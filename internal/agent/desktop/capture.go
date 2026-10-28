package desktop

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/jpeg"

	"gomesh/internal/protocol"

	"github.com/kbinani/screenshot"
)

// CaptureDisplay captures a screenshot of the specified monitor index and encodes as JPEG
func CaptureDisplay(displayIndex int, quality int) (*protocol.DesktopFramePayload, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, fmt.Errorf("no active displays detected")
	}

	if displayIndex < 0 || displayIndex >= n {
		displayIndex = 0
	}

	bounds := screenshot.GetDisplayBounds(displayIndex)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("failed capturing screen: %w", err)
	}

	if quality <= 0 || quality > 100 {
		quality = 65
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("failed encoding frame: %w", err)
	}

	b64Data := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &protocol.DesktopFramePayload{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Format: "jpeg",
		Data:   b64Data,
	}, nil
}

package desktop

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"gomesh/internal/protocol"

	"github.com/kbinani/screenshot"
)

var (
	ErrFrameUnchanged = errors.New("frame unchanged")

	hashMu         sync.Mutex
	lastScreenHash uint64
	lastFrameSent  time.Time
)

// fastScreenHash samples pixels across the image buffer to detect visual changes in < 0.2ms
func fastScreenHash(img *image.RGBA) uint64 {
	pix := img.Pix
	n := len(pix)
	if n == 0 {
		return 0
	}
	var h uint64 = 14695981039346656037
	step := 48 // sample every 12th pixel
	for i := 0; i < n; i += step {
		h ^= uint64(pix[i])
		h *= 1099511628211
	}
	return h
}

// CaptureDisplay captures a screenshot of the specified monitor index and encodes as JPEG
func CaptureDisplay(displayIndex int, quality int) (*protocol.DesktopFramePayload, error) {
	if quality <= 0 || quality > 100 {
		quality = 45
	}

	n := screenshot.NumActiveDisplays()
	if n > 0 {
		if displayIndex < 0 || displayIndex >= n {
			displayIndex = 0
		}

		bounds := screenshot.GetDisplayBounds(displayIndex)
		img, err := screenshot.CaptureRect(bounds)
		if err == nil && img != nil {
			hashMu.Lock()
			currentHash := fastScreenHash(img)
			now := time.Now()
			// If screen is unchanged and last frame was sent within 500ms, skip heavy encoding!
			if currentHash == lastScreenHash && now.Sub(lastFrameSent) < 500*time.Millisecond {
				hashMu.Unlock()
				return nil, ErrFrameUnchanged
			}
			lastScreenHash = currentHash
			lastFrameSent = now
			hashMu.Unlock()

			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err == nil {
				return &protocol.DesktopFramePayload{
					Width:  bounds.Dx(),
					Height: bounds.Dy(),
					Format: "jpeg",
					Data:   base64.StdEncoding.EncodeToString(buf.Bytes()),
				}, nil
			}
		}
	}

	// Fallback for macOS (darwin) when CGO is disabled or kbinani/screenshot cannot detect displays
	if runtime.GOOS == "darwin" {
		return captureDarwinCLI(quality)
	}

	return nil, fmt.Errorf("no active displays detected (ensure display is active and screen recording permission is granted)")
}

// captureDarwinCLI uses the built-in macOS /usr/sbin/screencapture command as a robust fallback
func captureDarwinCLI(quality int) (*protocol.DesktopFramePayload, error) {
	screencapturePath, err := exec.LookPath("screencapture")
	if err != nil {
		screencapturePath = "/usr/sbin/screencapture"
	}

	tmpFile := fmt.Sprintf("/tmp/gomesh_cap_%d.jpg", os.Getpid())
	defer os.Remove(tmpFile)

	// -x: mute sound, -t jpg: jpeg format, -m: main monitor
	cmd := exec.Command(screencapturePath, "-x", "-t", "jpg", "-m", tmpFile)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("screencapture failed: %w (check Screen Recording permission in System Settings)", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed reading captured frame: %w", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	width, height := 1920, 1080
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		width, height = cfg.Width, cfg.Height
	}

	return &protocol.DesktopFramePayload{
		Width:  width,
		Height: height,
		Format: "jpeg",
		Data:   base64.StdEncoding.EncodeToString(data),
	}, nil
}

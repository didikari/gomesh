//go:build linux

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"gomesh/internal/protocol"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

var (
	xConnMu sync.Mutex
	xConn   *xgb.Conn
	xRoot   xproto.Window
	xInited bool
)

func getXConn() (*xgb.Conn, xproto.Window, error) {
	xConnMu.Lock()
	defer xConnMu.Unlock()

	if xInited && xConn != nil {
		return xConn, xRoot, nil
	}

	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
		os.Setenv("DISPLAY", ":0")
	}

	c, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, 0, err
	}

	if err := xtest.Init(c); err != nil {
		c.Close()
		return nil, 0, err
	}

	setup := xproto.Setup(c)
	screen := setup.DefaultScreen(c)
	xConn = c
	xRoot = screen.Root
	xInited = true

	return xConn, xRoot, nil
}

func injectInput(payload protocol.DesktopInputPayload) error {
	// 1. Try pure Go X11 / XTest protocol
	if c, root, err := getXConn(); err == nil && c != nil {
		switch payload.Event {
		case "mousemove":
			xtest.FakeInput(c, 6, 0, 0, root, int16(payload.X), int16(payload.Y), 0)
			return nil

		case "mousedown":
			btn := byte(1)
			if payload.Button == 2 {
				btn = 3 // X11 right click
			} else if payload.Button == 1 {
				btn = 2 // X11 middle click
			}
			xtest.FakeInput(c, 6, 0, 0, root, int16(payload.X), int16(payload.Y), 0)
			xtest.FakeInput(c, 4, btn, 0, root, int16(payload.X), int16(payload.Y), 0) // ButtonPress
			return nil

		case "mouseup":
			btn := byte(1)
			if payload.Button == 2 {
				btn = 3
			} else if payload.Button == 1 {
				btn = 2
			}
			xtest.FakeInput(c, 5, btn, 0, root, int16(payload.X), int16(payload.Y), 0) // ButtonRelease
			return nil

		case "click":
			btn := byte(1)
			if payload.Button == 2 {
				btn = 3
			} else if payload.Button == 1 {
				btn = 2
			}
			xtest.FakeInput(c, 6, 0, 0, root, int16(payload.X), int16(payload.Y), 0)
			xtest.FakeInput(c, 4, btn, 0, root, int16(payload.X), int16(payload.Y), 0)
			xtest.FakeInput(c, 5, btn, 0, root, int16(payload.X), int16(payload.Y), 0)
			return nil

		case "wheel":
			btn := byte(5) // Scroll down
			if payload.DeltaY < 0 {
				btn = 4 // Scroll up
			}
			if payload.X > 0 || payload.Y > 0 {
				xtest.FakeInput(c, 6, 0, 0, root, int16(payload.X), int16(payload.Y), 0)
			}
			xtest.FakeInput(c, 4, btn, 0, root, int16(payload.X), int16(payload.Y), 0)
			xtest.FakeInput(c, 5, btn, 0, root, int16(payload.X), int16(payload.Y), 0)
			return nil
		}
	}

	// 2. Fallback to CLI utilities (xdotool / ydotool)
	return injectLinuxCLI(payload)
}

func injectLinuxCLI(payload protocol.DesktopInputPayload) error {
	switch payload.Event {
	case "mousemove":
		if path, err := exec.LookPath("xdotool"); err == nil {
			_ = exec.Command(path, "mousemove", fmt.Sprint(int(payload.X)), fmt.Sprint(int(payload.Y))).Run()
		}
	case "click", "mousedown":
		btn := "1"
		if payload.Button == 2 {
			btn = "3"
		} else if payload.Button == 1 {
			btn = "2"
		}
		if path, err := exec.LookPath("xdotool"); err == nil {
			_ = exec.Command(path, "mousemove", fmt.Sprint(int(payload.X)), fmt.Sprint(int(payload.Y)), "click", btn).Run()
		}
	case "wheel":
		btn := "5"
		if payload.DeltaY < 0 {
			btn = "4"
		}
		if path, err := exec.LookPath("xdotool"); err == nil {
			_ = exec.Command(path, "mousemove", fmt.Sprint(int(payload.X)), fmt.Sprint(int(payload.Y)), "click", btn).Run()
		}
	}
	return nil
}

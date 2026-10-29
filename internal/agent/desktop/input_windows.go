//go:build windows

package desktop

import (
	"syscall"

	"gomesh/internal/protocol"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procSetCursorPos = user32.NewProc("SetCursorPos")
	procMouseEvent   = user32.NewProc("mouse_event")
	procKeybdEvent   = user32.NewProc("keybd_event")
)

const (
	MOUSEEVENTF_LEFTDOWN   = 0x0002
	MOUSEEVENTF_LEFTUP     = 0x0004
	MOUSEEVENTF_RIGHTDOWN  = 0x0008
	MOUSEEVENTF_RIGHTUP    = 0x0010
	MOUSEEVENTF_MIDDLEDOWN = 0x0020
	MOUSEEVENTF_MIDDLEUP   = 0x0040
	MOUSEEVENTF_WHEEL      = 0x0800

	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_UNICODE     = 0x0004
)

var specialKeys = map[string]uintptr{
	"Enter":      0x0D,
	"Backspace":  0x08,
	"Tab":        0x09,
	"Escape":     0x1B,
	"Delete":     0x2E,
	"ArrowUp":    0x26,
	"ArrowDown":  0x28,
	"ArrowLeft":  0x25,
	"ArrowRight": 0x27,
	"Home":       0x24,
	"End":        0x23,
	"PageUp":     0x21,
	"PageDown":   0x22,
	"Shift":      0x10,
	"Control":    0x11,
	"Alt":        0x12,
	"Meta":       0x5B,
	"CapsLock":   0x14,
}

func injectKeyWindows(key string, isUp bool) {
	if key == "" {
		return
	}

	var flags uintptr
	if isUp {
		flags |= KEYEVENTF_KEYUP
	}

	if vk, ok := specialKeys[key]; ok {
		if key == "ArrowUp" || key == "ArrowDown" || key == "ArrowLeft" || key == "ArrowRight" ||
			key == "Home" || key == "End" || key == "PageUp" || key == "PageDown" || key == "Delete" || key == "Meta" {
			flags |= KEYEVENTF_EXTENDEDKEY
		}
		_, _, _ = procKeybdEvent.Call(vk, 0, flags, 0)
		return
	}

	runes := []rune(key)
	if len(runes) == 1 {
		flags |= KEYEVENTF_UNICODE
		_, _, _ = procKeybdEvent.Call(0, uintptr(runes[0]), flags, 0)
	}
}

func injectInput(payload protocol.DesktopInputPayload) error {
	switch payload.Event {
	case "mousemove":
		_, _, _ = procSetCursorPos.Call(uintptr(int(payload.X)), uintptr(int(payload.Y)))

	case "mousedown":
		_, _, _ = procSetCursorPos.Call(uintptr(int(payload.X)), uintptr(int(payload.Y)))
		flag := uintptr(MOUSEEVENTF_LEFTDOWN)
		if payload.Button == 2 {
			flag = MOUSEEVENTF_RIGHTDOWN
		} else if payload.Button == 1 {
			flag = MOUSEEVENTF_MIDDLEDOWN
		}
		_, _, _ = procMouseEvent.Call(flag, 0, 0, 0, 0)

	case "mouseup":
		_, _, _ = procSetCursorPos.Call(uintptr(int(payload.X)), uintptr(int(payload.Y)))
		flag := uintptr(MOUSEEVENTF_LEFTUP)
		if payload.Button == 2 {
			flag = MOUSEEVENTF_RIGHTUP
		} else if payload.Button == 1 {
			flag = MOUSEEVENTF_MIDDLEUP
		}
		_, _, _ = procMouseEvent.Call(flag, 0, 0, 0, 0)

	case "click":
		_, _, _ = procSetCursorPos.Call(uintptr(int(payload.X)), uintptr(int(payload.Y)))
		down := uintptr(MOUSEEVENTF_LEFTDOWN)
		up := uintptr(MOUSEEVENTF_LEFTUP)
		if payload.Button == 2 {
			down = MOUSEEVENTF_RIGHTDOWN
			up = MOUSEEVENTF_RIGHTUP
		}
		_, _, _ = procMouseEvent.Call(down, 0, 0, 0, 0)
		_, _, _ = procMouseEvent.Call(up, 0, 0, 0, 0)

	case "wheel":
		if payload.X > 0 || payload.Y > 0 {
			_, _, _ = procSetCursorPos.Call(uintptr(int(payload.X)), uintptr(int(payload.Y)))
		}
		var delta int32 = -120
		if payload.DeltaY < 0 {
			delta = 120
		}
		_, _, _ = procMouseEvent.Call(uintptr(MOUSEEVENTF_WHEEL), 0, 0, uintptr(uint32(delta)), 0)

	case "keydown":
		injectKeyWindows(payload.Key, false)

	case "keyup":
		injectKeyWindows(payload.Key, true)
	}

	return nil
}

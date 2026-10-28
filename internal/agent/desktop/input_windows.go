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
)

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
		_, _, _ = procMouseEvent.Call(uintptr(MOUSEEVENTF_WHEEL), 0, 0, uintptr(int(payload.DeltaY)), 0)
	}

	return nil
}

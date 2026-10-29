//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	mutexHandle     uintptr
)

const errAlreadyExists = 183

func acquireSingleInstanceLock() (bool, func()) {
	name, _ := syscall.UTF16PtrFromString("Global\\GoMeshAgentSingleInstanceLock")
	ret, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if ret == 0 {
		localName, _ := syscall.UTF16PtrFromString("Local\\GoMeshAgentSingleInstanceLock")
		ret, _, err = procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(localName)))
		if ret == 0 {
			return true, func() {}
		}
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errAlreadyExists {
		syscall.CloseHandle(syscall.Handle(ret))
		return false, nil
	}
	mutexHandle = ret
	return true, func() {
		if mutexHandle != 0 {
			syscall.CloseHandle(syscall.Handle(mutexHandle))
			mutexHandle = 0
		}
	}
}

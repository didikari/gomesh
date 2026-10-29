//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

var lockFile *os.File

func acquireSingleInstanceLock() (bool, func()) {
	lockPath := filepath.Join(os.TempDir(), "gomesh-agent.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return true, func() {}
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		return false, nil
	}
	lockFile = f
	return true, func() {
		if lockFile != nil {
			_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
			_ = lockFile.Close()
			_ = os.Remove(lockPath)
			lockFile = nil
		}
	}
}

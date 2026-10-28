//go:build !windows

package main

func hideConsoleWindow() {}

func checkAndSelfInstall(isService bool, foreground bool) (shouldExit bool) {
	return false
}

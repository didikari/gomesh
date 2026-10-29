//go:build !windows

package main

import (
	"log/slog"
	"os"
	"os/exec"
)

func uninstallAgent() {
	slog.Info("uninstalling GoMesh agent...")
	_ = exec.Command("systemctl", "stop", "gomesh-agent").Run()
	_ = exec.Command("systemctl", "disable", "gomesh-agent").Run()
	_ = os.Remove("/etc/systemd/system/gomesh-agent.service")
	_ = os.RemoveAll("/opt/gomesh")
	slog.Info("GoMesh agent uninstalled cleanly")
	os.Exit(0)
}

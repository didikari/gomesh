//go:build windows

package main

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func uninstallAgent() {
	slog.Info("uninstalling GoMesh agent from system...")

	stopCmd := exec.Command("sc.exe", "stop", "GoMeshAgent")
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = stopCmd.Run()

	delCmd := exec.Command("sc.exe", "delete", "GoMeshAgent")
	delCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = delCmd.Run()

	schCmd := exec.Command("schtasks.exe", "/delete", "/tn", "GoMeshAgent", "/f")
	schCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = schCmd.Run()

	regCmd := exec.Command("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "GoMeshAgent", "/f")
	regCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = regCmd.Run()

	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = "C:\\ProgramData"
	}
	targetDir := filepath.Join(programData, "GoMesh")
	_ = os.Remove(filepath.Join(targetDir, "gomesh-agent.json"))
	_ = os.Remove(filepath.Join(targetDir, "gomesh-agent.exe.old"))

	showWindowsNotification(
		"GoMesh Remote Agent",
		"GoMesh Agent berhasil di-uninstall!\n\nSeluruh service dan background task telah dihentikan dan dihapus dari sistem.",
	)
	os.Exit(0)
}

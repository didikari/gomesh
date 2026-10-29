//go:build windows

package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func hideConsoleWindow() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		showWindow.Call(hwnd, 0) // SW_HIDE = 0
	}
}

func showWindowsNotification(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(message)

	// MB_OK (0x0) | MB_ICONINFORMATION (0x40) | MB_TOPMOST (0x40000)
	messageBox.Call(0, uintptr(unsafe.Pointer(msgPtr)), uintptr(unsafe.Pointer(titlePtr)), 0x00000040|0x00040000)
}

func checkAndSelfInstall(isService bool, foreground bool) (shouldExit bool) {
	// If running as a registered Windows Service, or explicitly requested foreground mode, do not self-install
	if isService || foreground {
		return false
	}

	execPath, err := os.Executable()
	if err != nil {
		return false
	}
	realExecPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realExecPath
	}

	// Determine permanent installation directory
	// Use C:\ProgramData\GoMesh (writable by users, accessible across all accounts)
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = "C:\\ProgramData"
	}
	targetDir := filepath.Join(programData, "GoMesh")
	targetExe := filepath.Join(targetDir, "gomesh-agent.exe")

	// If ProgramData fails, fallback to %LocalAppData%\GoMesh
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		localAppData := os.Getenv("LocalAppData")
		if localAppData != "" {
			targetDir = filepath.Join(localAppData, "GoMesh")
			targetExe = filepath.Join(targetDir, "gomesh-agent.exe")
			_ = os.MkdirAll(targetDir, 0755)
		}
	}

	// Check if already running from permanent installation directory
	if strings.EqualFold(filepath.Clean(execPath), filepath.Clean(targetExe)) {
		// Running permanently in background
		hideConsoleWindow()
		return false
	}

	// We were executed from temporary folder (Downloads, Desktop, USB)
	hideConsoleWindow()

	slog.Info("self-installing agent to permanent background directory", "target", targetExe)

	// Copy current executable (including baked-in configuration overlay!) to permanent target
	if err := copyFile(execPath, targetExe); err != nil {
		slog.Warn("could not copy executable to permanent path, running from current location", "error", err)
		return false
	}

	serviceInstalled := false
	binPathArg := fmt.Sprintf("\"%s\" -config \"%s\"", targetExe, targetDir)

	queryCmd := exec.Command("sc.exe", "query", "GoMeshAgent")
	queryCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if queryCmd.Run() == nil {
		cfgCmd := exec.Command("sc.exe", "config", "GoMeshAgent", "binPath=", binPathArg)
		cfgCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		_ = cfgCmd.Run()

		startCmd := exec.Command("sc.exe", "start", "GoMeshAgent")
		startCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		_ = startCmd.Run()
		serviceInstalled = true
		slog.Info("updated and started existing Windows Service GoMeshAgent successfully")
	} else {
		scCmd := exec.Command("sc.exe", "create", "GoMeshAgent", "binPath=", binPathArg, "start=", "auto", "DisplayName=", "GoMesh Remote Management Agent")
		scCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if err := scCmd.Run(); err == nil {
			_ = exec.Command("sc.exe", "description", "GoMeshAgent", "GoMesh background agent for secure remote device management.").Run()
			_ = exec.Command("sc.exe", "failure", "GoMeshAgent", "reset=", "86400", "actions=", "restart/5000/restart/10000/restart/60000").Run()
			startCmd := exec.Command("sc.exe", "start", "GoMeshAgent")
			startCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			if err := startCmd.Run(); err == nil {
				serviceInstalled = true
				slog.Info("registered and started as Windows Service successfully")
			}
		}
	}

	if !serviceInstalled {
		schCmd := exec.Command("schtasks.exe", "/create", "/tn", "GoMeshAgent", "/tr", fmt.Sprintf("\"%s\" -config \"%s\"", targetExe, targetDir), "/sc", "onlogon", "/rl", "highest", "/f")
		schCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		_ = schCmd.Run()

		regCmd := exec.Command("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "GoMeshAgent", "/t", "REG_SZ", "/d", fmt.Sprintf("\"%s\" -config \"%s\"", targetExe, targetDir), "/f")
		regCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		_ = regCmd.Run()

		runCmd := exec.Command(targetExe, "-config", targetDir)
		runCmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
		}
		if err := runCmd.Start(); err != nil {
			slog.Error("failed starting detached background process", "error", err)
		} else {
			slog.Info("detached background agent started successfully")
		}
	}

	// Notify the user with native Windows Dialog
	showWindowsNotification(
		"GoMesh Remote Agent",
		"GoMesh Agent berhasil diaktifkan!\n\n"+
			"Aplikasi sekarang berjalan di latar belakang (Background) dan akan otomatis menyala setiap kali komputer dihidupkan.",
	)

	return true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Stop running service if present so binary lock is released
	stopCmd := exec.Command("sc.exe", "stop", "GoMeshAgent")
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = stopCmd.Run()

	// Windows file replace trick: Rename existing executable to .old (succeeds even if locked)
	oldPath := dst + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(dst, oldPath); err != nil {
		_ = os.Remove(dst)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

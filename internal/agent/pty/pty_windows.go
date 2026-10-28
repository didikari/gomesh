//go:build windows

package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type windowsPipePTY struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (p *windowsPipePTY) Read(b []byte) (int, error) {
	return p.stdout.Read(b)
}

func (p *windowsPipePTY) Write(b []byte) (int, error) {
	return p.stdin.Write(b)
}

func (p *windowsPipePTY) Close() error {
	_ = p.stdin.Close()
	_ = p.stdout.Close()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
	}
	return nil
}

func (p *windowsPipePTY) Resize(cols, rows uint16) error {
	// Fallback pipe does not support window resize signals
	return nil
}

// Start launches powershell.exe or cmd.exe on Windows
func Start(cols, rows uint16) (PTY, error) {
	shell := "powershell.exe"
	if _, err := exec.LookPath(shell); err != nil {
		shell = "cmd.exe"
	}

	cmd := exec.Command(shell)
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed creating stdout pipe: %w", err)
	}

	// Route stderr to stdout
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to start windows shell: %w", err)
	}

	return &windowsPipePTY{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}, nil
}

//go:build !windows

package pty

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixPTY struct {
	file *os.File
	cmd  *exec.Cmd
}

func (p *unixPTY) Read(b []byte) (int, error) {
	return p.file.Read(b)
}

func (p *unixPTY) Write(b []byte) (int, error) {
	return p.file.Write(b)
}

func (p *unixPTY) Close() error {
	_ = p.file.Close()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
	}
	return nil
}

func (p *unixPTY) Resize(cols, rows uint16) error {
	ws := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}
	return pty.Setsize(p.file, ws)
}

// Start launches a new interactive shell session in a pseudo-terminal
func Start(cols, rows uint16) (PTY, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		candidates := []string{"/bin/bash", "/bin/zsh", "/bin/sh"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				shell = c
				break
			}
		}
	}
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
	)

	ws := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}
	if ws.Rows == 0 {
		ws.Rows = 24
	}
	if ws.Cols == 0 {
		ws.Cols = 80
	}

	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty with shell %s: %w", shell, err)
	}

	return &unixPTY{
		file: ptmx,
		cmd:  cmd,
	}, nil
}

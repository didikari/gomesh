package pty

import "io"

// PTY defines the cross-platform pseudo-terminal interface
type PTY interface {
	io.ReadWriteCloser
	Resize(cols, rows uint16) error
}

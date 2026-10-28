//go:build !windows

package main

import "context"

func runAsWindowsService(name string, runFunc func(ctx context.Context) error) (bool, error) {
	return false, nil
}

//go:build windows
// +build windows

package core

import (
	"syscall"
)

// GetSIGUSR1 returns a dummy signal for Windows (SIGUSR1 doesn't exist on Windows)
func GetSIGUSR1() syscall.Signal {
	// Return a valid signal that exists on Windows
	return syscall.SIGINT
}
//go:build !windows
// +build !windows

package core

import (
	"syscall"
)

// GetSIGUSR1 returns SIGUSR1 signal for Unix-like systems
func GetSIGUSR1() syscall.Signal {
	return syscall.SIGUSR1
}

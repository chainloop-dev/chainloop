//go:build !windows

package claude

import "syscall"

// isPIDAlive reports whether a process with the given PID exists by sending signal 0.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	return syscall.Kill(pid, 0) == nil
}

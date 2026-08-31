//go:build windows

package claude

import "os"

// isPIDAlive reports whether a process with the given PID exists.
// On Windows os.FindProcess opens a handle to the process and fails if it does not exist,
// unlike on Unix where it always succeeds.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer proc.Release()

	return true
}

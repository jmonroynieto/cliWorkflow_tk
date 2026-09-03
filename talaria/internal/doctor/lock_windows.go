//go:build windows

package doctor

import (
	"fmt"
	"os"
)

func probeLockHeld(lockPath string) (held bool, err error) {
	// Registry locking is not implemented on Windows; report unknown as free.
	if _, err := os.Stat(lockPath); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("stat lock %s: %w", lockPath, err)
	}
	return false, nil
}

func signalZero(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows FindProcess does not prove liveness; best-effort.
	return p != nil
}

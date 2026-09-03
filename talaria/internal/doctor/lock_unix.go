//go:build unix

package doctor

import (
	"fmt"
	"os"
	"syscall"
)

// probeLockHeld returns whether another process currently holds the exclusive flock.
// If the lock is free, this process briefly acquires and releases it.
func probeLockHeld(lockPath string) (held bool, err error) {
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false, fmt.Errorf("open lock %s: %w", lockPath, err)
	}
	defer lf.Close()

	err = syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		_ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
		return false, nil
	}
	if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
		return true, nil
	}
	return false, fmt.Errorf("flock %s: %w", lockPath, err)
}

func signalZero(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

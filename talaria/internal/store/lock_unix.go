//go:build unix

package store

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

// flockExclusive blocks until an exclusive advisory lock is held on f.
func flockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// flockExclusiveContext tries non-blocking locks until success or ctx done.
func flockExclusiveContext(ctx context.Context, f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for registry lock: %w", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

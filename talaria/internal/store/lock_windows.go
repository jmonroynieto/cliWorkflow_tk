//go:build windows

package store

import (
	"context"
	"fmt"
	"os"
)

func flockExclusive(f *os.File) error {
	return fmt.Errorf("registry file locking is not implemented on windows")
}

func flockExclusiveContext(ctx context.Context, f *os.File) error {
	return flockExclusive(f)
}

func flockUnlock(f *os.File) error {
	return nil
}

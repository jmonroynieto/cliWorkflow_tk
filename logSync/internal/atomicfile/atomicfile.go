// Package atomicfile writes files via temp-file-then-rename, so a reader
// (or an interrupted logSync run) never observes a half-written file.
// Mirrors the pattern the bash tool used (mktemp in the same dir + mv) and
// that janus's writeFileAtomic already establishes as this codebase's
// convention.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultMode is used when dest does not already exist, so we have no prior
// mode to preserve.
const defaultMode = 0o644

// Write atomically replaces dest with data, preserving dest's existing file
// mode if it exists (matching the bash merge helper's
// `stat -c %a "$dest" && chmod "$mode" "$tmp"`).
func Write(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	mode := os.FileMode(defaultMode)
	if info, err := os.Stat(dest); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(dest)+".logSync-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, dest, err)
	}
	return nil
}

// CopyPreservingTimestamp atomically copies src to dest, preserving src's
// mtime (matching the bash copy helper's `cp --preserve=timestamps`).
func CopyPreservingTimestamp(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if err := Write(dest, data); err != nil {
		return err
	}
	if err := os.Chtimes(dest, info.ModTime(), info.ModTime()); err != nil {
		return fmt.Errorf("preserve mtime on %s: %w", dest, err)
	}
	return nil
}

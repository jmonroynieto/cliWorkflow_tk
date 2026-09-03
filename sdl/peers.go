package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// printPeers finds and prints the other paths hard-linked to the same
// inode as path, searching from path's filesystem mount point downward
// (never crossing into other filesystems, since hard links cannot span
// filesystems). This mirrors:
//
//	find "$(df -P -- "$f" | awk 'NR==2{print $6}')" -xdev -samefile "$f"
func printPeers(path string, target *syscall.Stat_t) {
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdl: cannot resolve %q: %v\n", path, err)
		return
	}

	mount, err := findMountPoint(abs, uint64(target.Dev))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdl: cannot locate mount point for %q: %v\n", path, err)
		return
	}

	dev := uint64(target.Dev)
	ino := target.Ino

	_ = filepath.WalkDir(mount, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil
		}
		if uint64(st.Dev) != dev {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if st.Ino == ino && p != abs {
			fmt.Printf("  also linked: %s\n", p)
		}
		return nil
	})
}

// findMountPoint walks up from the directory containing file until the
// device ID changes, returning the highest ancestor directory that still
// shares dev. That directory is file's mount point.
func findMountPoint(file string, dev uint64) (string, error) {
	cur := filepath.Dir(file)
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		pInfo, err := os.Lstat(parent)
		if err != nil {
			break
		}
		pSt, ok := pInfo.Sys().(*syscall.Stat_t)
		if !ok || uint64(pSt.Dev) != dev {
			break
		}
		cur = parent
	}
	return cur, nil
}

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

var errRefused = errors.New("refused")

// processArg dispatches a single command-line argument: a directory (only
// valid with -r), or anything else (file, symlink, etc.) via removeSingle.
func processArg(path string, opts *options) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.force {
				return nil
			}
			fmt.Fprintf(os.Stderr, "sdl: cannot remove %q: No such file or directory\n", path)
			return err
		}
		fmt.Fprintf(os.Stderr, "sdl: cannot remove %q: %v\n", path, err)
		return err
	}

	if info.IsDir() {
		if !opts.recursive {
			fmt.Fprintf(os.Stderr, "sdl: cannot remove %q: Is a directory (use -r to recurse into it)\n", path)
			return errors.New("is a directory")
		}
		return removeTree(path, opts)
	}

	return removeSingle(path, info, opts)
}

// removeSingle applies the hard-link safety check to one file and removes
// it if safe. Symlinks are always safe to remove, since deleting a symlink
// never touches the data it points to.
func removeSingle(path string, info fs.FileInfo, opts *options) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return doRemove(path, opts, "symlink")
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		fmt.Fprintf(os.Stderr, "sdl: cannot inspect %q: unsupported platform\n", path)
		return errors.New("unsupported platform")
	}

	if st.Nlink <= 1 {
		fmt.Fprintf(os.Stderr, "sdl: refusing to remove %q: only link left (nlink=1) -- would delete data\n", path)
		return errRefused
	}

	if opts.showLinks {
		printPeers(path, st)
	}

	return doRemove(path, opts, fmt.Sprintf("nlink=%d", st.Nlink))
}

// doRemove handles the interactive prompt, dry-run reporting, verbose
// reporting, and the actual unlink, shared by both single-file and
// tree-recursion paths.
func doRemove(path string, opts *options, detail string) error {
	if opts.interactive && !confirm(path) {
		return nil
	}
	if opts.dryRun {
		fmt.Printf("would remove %q (%s)\n", path, detail)
		return nil
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(os.Stderr, "sdl: cannot remove %q: %v\n", path, err)
		return err
	}
	if opts.verbose {
		fmt.Printf("removed %q (%s)\n", path, detail)
	}
	return nil
}

func confirm(path string) bool {
	fmt.Fprintf(os.Stderr, "sdl: remove %q? ", path)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

type fileEntry struct {
	path string
	dev  uint64
	ino  uint64
}

// removeTree walks a directory tree, removing every regular file it finds
// whose hard-link count is greater than 1. Directories and symlinks
// encountered during the walk are left untouched; directories are never
// removed by sdl, and non-regular entries (symlinks, devices, sockets) are
// skipped, matching `find -type f`.
func removeTree(root string, opts *options) error {
	rootStat, err := os.Lstat(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdl: cannot access %q: %v\n", root, err)
		return err
	}
	rootSt, ok := rootStat.Sys().(*syscall.Stat_t)
	if !ok {
		fmt.Fprintf(os.Stderr, "sdl: cannot inspect %q: unsupported platform\n", root)
		return errors.New("unsupported platform")
	}
	rootDev := uint64(rootSt.Dev)

	var files []fileEntry
	failed := false

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "sdl: %v\n", err)
			failed = true
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sdl: cannot stat %q: %v\n", p, err)
			failed = true
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil
		}

		if d.IsDir() {
			if p != root && opts.xdev && uint64(st.Dev) != rootDev {
				return filepath.SkipDir
			}
			return nil
		}

		if opts.xdev && uint64(st.Dev) != rootDev {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}

		files = append(files, fileEntry{path: p, dev: uint64(st.Dev), ino: st.Ino})
		return nil
	})
	if walkErr != nil {
		fmt.Fprintf(os.Stderr, "sdl: %v\n", walkErr)
		failed = true
	}

	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	var peers map[[2]uint64][]string
	if opts.showLinks {
		peers = make(map[[2]uint64][]string, len(files))
		for _, fe := range files {
			key := [2]uint64{fe.dev, fe.ino}
			peers[key] = append(peers[key], fe.path)
		}
	}

	for _, fe := range files {
		// Re-stat right before acting: the walk may have taken a while on a
		// large tree, and nlink can change out from under us in the meantime.
		info, err := os.Lstat(fe.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "sdl: cannot stat %q: %v\n", fe.path, err)
			failed = true
			continue
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		if st.Nlink <= 1 {
			fmt.Fprintf(os.Stderr, "sdl: refusing to remove %q: only link left (nlink=1) -- would delete data\n", fe.path)
			failed = true
			continue
		}

		if opts.showLinks {
			for _, pp := range peers[[2]uint64{fe.dev, fe.ino}] {
				if pp != fe.path {
					fmt.Printf("  also linked: %s\n", pp)
				}
			}
		}

		if err := doRemove(fe.path, opts, fmt.Sprintf("nlink=%d", st.Nlink)); err != nil {
			failed = true
		}
	}

	if failed {
		return errors.New("one or more files were refused or failed")
	}
	return nil
}

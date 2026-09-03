package adbx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/electricbubble/gadb"
)

// StageTimeout is the floor on how long one file transfer may take. Kept
// separate from DefaultTimeout (used for single metadata calls) since a
// legitimately large file takes longer than a LIST/STAT round trip should.
const StageTimeout = 30 * time.Second

// stageThroughput is the slowest transfer rate a healthy adb link is
// expected to sustain, used to grow the per-file deadline with the file.
// Measured on a USB-attached tablet: 40 MB moved in 2.9 s, about 13 MB/s,
// so 1 MB/s leaves better than an order of magnitude of headroom while
// still bounding a genuinely stuck transfer. A flat 30 s used to be the
// whole budget, which meant any attachment past roughly 400 MB — a video,
// a scanned book — timed out on a link that was working fine.
const stageThroughput = 1 << 20 // bytes per second

// shortcutMinSize is the size below which StageIn always pulls the real
// bytes instead of trusting size+mtime and reusing the local copy.
//
// The trust model has a hole: two files with the same length and the same
// mtime-to-the-second are assumed identical, so a note edited on both sides
// within one second, ending at the same length, reads as "already in sync"
// and the phone's version is invisible — and then overwritten by the next
// push. Toggling `- [ ]` to `- [x]` on both devices is exactly that shape.
//
// The shortcut exists to avoid re-downloading large attachments, where the
// saving is seconds per file and same-second concurrent edits are not a
// thing that happens. For a small note the saving is one round trip and the
// risk is a lost edit, so those are always fetched. Above this size the
// residual risk remains and is the price of not moving a 40 MB scan on
// every run.
const shortcutMinSize = 1 << 20

// transferTimeout is StageTimeout for a small file, growing with size for
// a large one.
func transferTimeout(size int64) time.Duration {
	d := StageTimeout + time.Duration(size/stageThroughput)*time.Second
	if d < StageTimeout {
		return StageTimeout
	}
	return d
}

// remoteFile is one entry from a recursive remote listing, keeping the
// metadata gadb.List already returned instead of discarding it down to a
// bare path — StageIn's local-shortcut check (below) needs Size and
// LastModified without paying for a second round trip per file.
type remoteFile struct {
	Rel          string
	Size         uint32
	LastModified time.Time
}

// listRecursive walks remoteDir on the device, returning every regular
// file's path relative to remoteDir, with the size/mtime gadb's LIST already
// reported. gadb's List only lists one directory (see
// docs/upstream-gadb-notes.md item 1 — no STAT, and no recursive LIST
// either), so this recurses by hand.
func listRecursive(dev gadb.Device, remoteDir string) ([]remoteFile, error) {
	var out []remoteFile
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := WithTimeoutValue(DefaultTimeout, func() ([]gadb.DeviceFileInfo, error) {
			return dev.List(dir)
		})
		if err != nil {
			return fmt.Errorf("list %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.Name == "." || e.Name == ".." || e.Name == ".git" {
				continue
			}
			full := path.Join(dir, e.Name)
			if e.IsDir() {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}
			rel, err := filepath.Rel(remoteDir, full)
			if err != nil {
				return err
			}
			out = append(out, remoteFile{
				Rel:          filepath.ToSlash(rel),
				Size:         e.Size,
				LastModified: e.LastModified,
			})
		}
		return nil
	}
	if err := walk(remoteDir); err != nil {
		return nil, err
	}
	return out, nil
}

// StageResult reports the per-file outcome of a staging operation. A single
// file rejected by the device (e.g. an illegal character in the name) must
// never hide or block the transfer of every other file in the batch, and the
// caller needs to know exactly which paths actually moved — not just "it
// failed somewhere." Both used to abort entirely on the first per-file
// error, discarding that information.
type StageResult struct {
	Succeeded []string
	Failed    map[string]error
	// Shortcut counts entries in Succeeded that were satisfied by copying
	// the already-identical local file instead of downloading from the
	// device — see StageIn.
	Shortcut int
}

// StageIn pulls every file under remoteDir into stageDir, preserving
// relative paths and mtime. This replaces the bash tool's
// `adb exec-out tar` staging step — same
// "stage the whole tree locally, then compare on disk" shape, but over the
// sync protocol directly instead of a remote `tar`.
//
// realLocalDir is the actual vault directory this session will compare
// against (not the staging destination). gadb's LIST already reports each
// remote file's size and mtime at no extra cost (see listRecursive); when
// those match realLocalDir's copy of the same path exactly, that file is
// already known-identical, and StageIn copies it from realLocalDir instead
// of downloading it — the same size+mtime trust model `adb push --sync`
// relies on, which the bash tool's `notesync` mode already used. This is what
// keeps a run cheap once a large file (an attachment, say) has been synced
// once: every subsequent run used to re-download it in full just to learn
// it hadn't changed. Pass an empty realLocalDir to disable the shortcut
// entirely and always pull from the device. Files below shortcutMinSize are
// always pulled regardless — see that constant for why.
//
// onProgress, if non-nil, is called after each file that succeeds.
func StageIn(dev gadb.Device, remoteDir, stageDir, realLocalDir string, onProgress func(rel string)) (StageResult, error) {
	files, err := listRecursive(dev, remoteDir)
	if err != nil {
		return StageResult{}, fmt.Errorf("staging in from %s: %w", remoteDir, err)
	}

	res := StageResult{Failed: make(map[string]error)}
	for _, f := range files {
		shortcut, err := stageInOne(dev, remoteDir, stageDir, realLocalDir, f)
		if err != nil {
			res.Failed[f.Rel] = err
			continue
		}
		res.Succeeded = append(res.Succeeded, f.Rel)
		if shortcut {
			res.Shortcut++
		}
		if onProgress != nil {
			onProgress(f.Rel)
		}
	}
	return res, nil
}

// stageInOne stages one file, returning whether it was satisfied by the
// local-copy shortcut rather than a device pull.
func stageInOne(dev gadb.Device, remoteDir, stageDir, realLocalDir string, f remoteFile) (shortcut bool, err error) {
	remotePath := path.Join(remoteDir, f.Rel)
	stagePath := filepath.Join(stageDir, filepath.FromSlash(f.Rel))
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		return false, fmt.Errorf("mkdir for %s: %w", stagePath, err)
	}

	if realLocalDir != "" && f.Size >= shortcutMinSize {
		realPath := filepath.Join(realLocalDir, filepath.FromSlash(f.Rel))
		if info, statErr := os.Stat(realPath); statErr == nil && !info.IsDir() &&
			uint32(info.Size()) == f.Size && info.ModTime().Unix() == f.LastModified.Unix() {
			if err := copyFilePreservingMtime(realPath, stagePath, info.ModTime()); err == nil {
				return true, nil
			}
			// Local copy failed for some reason (permissions, vanished mid-run,
			// ...) — fall through and pull the real bytes from the device instead.
		}
	}

	var buf bytes.Buffer
	err = WithTimeout(transferTimeout(int64(f.Size)), func() error {
		return dev.Pull(remotePath, &buf)
	})
	if err != nil {
		return false, fmt.Errorf("pull %s: %w", remotePath, err)
	}
	if err := os.WriteFile(stagePath, buf.Bytes(), 0o644); err != nil {
		return false, fmt.Errorf("write staged file %s: %w", stagePath, err)
	}
	if err := os.Chtimes(stagePath, f.LastModified, f.LastModified); err != nil {
		return false, fmt.Errorf("set mtime on staged file %s: %w", stagePath, err)
	}
	return false, nil
}

func copyFilePreservingMtime(src, dst string, mtime time.Time) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, mtime, mtime)
}

// StageOut pushes the given relative paths from localDir to their
// counterparts under remoteDir, preserving each file's local mtime. Only
// the given paths are sent: only what actually changed is re-stamped,
// rather than blanket-copying the whole tree local→phone. onProgress, if
// non-nil, is called after each file that succeeds, so a caller can report
// progress live instead of only after the entire batch finishes.
func StageOut(dev gadb.Device, localDir, remoteDir string, relPaths []string, onProgress func(rel string)) (StageResult, error) {
	res := StageResult{Failed: make(map[string]error)}
	for _, rel := range relPaths {
		if err := stageOutOne(dev, localDir, remoteDir, rel); err != nil {
			res.Failed[rel] = err
			continue
		}
		res.Succeeded = append(res.Succeeded, rel)
		if onProgress != nil {
			onProgress(rel)
		}
	}
	return res, nil
}

// DeleteBatch removes each of relPaths from remoteDir on the device, used by
// --prune to retire files a caller has determined no longer belong there.
// Kept symmetric with StageOut: only the given paths are touched, and one
// failure doesn't block the rest of the batch.
//
// `rm -f` on Android's shell reports nothing back through the adb protocol —
// RunShellCommand only surfaces a transport error, never a nonzero exit
// status (see docs/upstream-gadb-notes.md) — so a permission error would
// otherwise look identical to success. The Stat check after the command is
// what actually confirms the file is gone; without it a failed delete would
// be reported as done and the stale file silently kept coming back as
// PhoneOnly every round after.
func DeleteBatch(dev gadb.Device, remoteDir string, relPaths []string, onProgress func(rel string)) (StageResult, error) {
	res := StageResult{Failed: make(map[string]error)}
	for _, rel := range relPaths {
		remotePath := path.Join(remoteDir, filepath.ToSlash(rel))
		out, err := WithTimeoutValue(DefaultTimeout, func() (string, error) {
			return dev.RunShellCommand("rm", "-f", shellQuote(remotePath))
		})
		if err != nil {
			res.Failed[rel] = fmt.Errorf("remove %s: %w", remotePath, err)
			continue
		}
		if msg := strings.TrimSpace(out); msg != "" {
			res.Failed[rel] = fmt.Errorf("remove %s: %s", remotePath, msg)
			continue
		}
		if _, stillThere, statErr := Stat(dev, remotePath); statErr == nil && stillThere {
			res.Failed[rel] = fmt.Errorf("remove %s: file still present after rm", remotePath)
			continue
		}
		res.Succeeded = append(res.Succeeded, rel)
		if onProgress != nil {
			onProgress(rel)
		}
	}
	return res, nil
}

func stageOutOne(dev gadb.Device, localDir, remoteDir, rel string) error {
	localPath := filepath.Join(localDir, filepath.FromSlash(rel))
	remotePath := path.Join(remoteDir, filepath.ToSlash(rel))

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat %s before push: %w", localPath, err)
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s before push: %w", localPath, err)
	}
	err = WithTimeout(transferTimeout(info.Size()), func() error {
		return dev.Push(bytes.NewReader(data), remotePath, info.ModTime())
	})
	if err != nil {
		return fmt.Errorf("push %s: %w", remotePath, err)
	}
	return nil
}

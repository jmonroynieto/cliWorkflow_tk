// Package synctree walks and classifies a pair of local directory trees
// (the real local tree and a staged local copy of the phone's tree) and
// provides the single-instance lock, reimplementing the file-selection and
// flock guard of the bash tool this replaces.
package synctree

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/relevance"
	"golang.org/x/sys/unix"
)

// WalkFiles returns the set of regular-file relative paths under root whose
// content can be compared byte-for-byte, and separately the set of every
// local symlink encountered — resolvable or not. Symlinks are always
// reported in the second set, regardless of includeSymlinks, because that
// set is a guardrail: a path that is currently a local symlink must never
// become a target for a phone-to-local write, whether or not this run
// chose to treat its content as syncable. includeSymlinks only controls
// whether a symlink that resolves to a regular file is *also* added to the
// first (comparable-content) set — a symlink to a directory, or a broken
// symlink, is never added there, since there is nothing to compare or send.
//
// Skips anything under a .git directory (matching
// `find . -type f ! -path '*/.git*'`) and anything under excludePaths —
// root-relative, forward-slash paths naming repo-management files/dirs
// (e.g. ".gitconfig", ".githooks") that live alongside vault content but
// were never meant to leave this machine. A directory match prunes the
// whole subtree; a file match skips just that file.
func WalkFiles(root string, excludePaths []string, includeSymlinks bool) (files map[string]struct{}, symlinks map[string]struct{}, broken map[string]struct{}, err error) {
	ex, err := newExcluder(excludePaths)
	if err != nil {
		return nil, nil, nil, err
	}

	files = make(map[string]struct{})
	symlinks = make(map[string]struct{})
	broken = make(map[string]struct{})
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if rel != "." && ex.match(relSlash) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(rel, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		// A `.git` that is a file rather than a directory is a gitdir
		// pointer — what a submodule or a linked worktree leaves behind,
		// holding a path that is only meaningful on this machine. The
		// directory check above skips repositories; this skips their
		// pointers. An Obsidian vault that keeps .obsidian/ as a submodule
		// has exactly one of these, and sending it to the phone is at best
		// meaningless and at worst a broken pointer coming back.
		if d.Name() == ".git" {
			return nil
		}
		if ex.match(relSlash) {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			symlinks[relSlash] = struct{}{}
			if !includeSymlinks {
				return nil
			}
			info, statErr := os.Stat(path) // follows the link
			if statErr != nil {
				// Broken symlink, or a target this machine cannot reach:
				// guarded, not comparable, and worth saying so — this run
				// asked for symlinked content and cannot supply this one.
				broken[relSlash] = struct{}{}
				return nil
			}
			if info.IsDir() {
				return nil // symlink to a directory: out of scope, guarded but not compared
			}
			files[relSlash] = struct{}{}
			return nil
		}

		files[relSlash] = struct{}{}
		return nil
	})
	if walkErr != nil {
		return nil, nil, nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}
	return files, symlinks, broken, nil
}

// excluder decides which walked paths sit outside the synced tree.
//
// Matching used to be an exact, root-anchored string compare, which meant
// `.trash` excluded only a `.trash` at the vault root while a nested one
// synced, and there was no way to write the `**/.trash/` rule the vault's
// own .gitignore already used. The rules now follow the shape people expect
// from a .gitignore:
//
//   - a bare name (`.trash`, `*.tmp`) matches at any depth;
//   - a pattern with a slash (`.obsidian/workspace.json`) is anchored at
//     the root;
//   - a `**/` prefix makes a multi-segment pattern match at any depth;
//   - `*` and `?` glob within one path segment, never across a `/`.
//
// A directory that matches prunes its whole subtree, as before.
type excluder struct {
	patterns []excludePattern
}

type excludePattern struct {
	pattern  string
	anchored bool
}

func newExcluder(excludePaths []string) (excluder, error) {
	var e excluder
	for _, raw := range excludePaths {
		p := strings.Trim(filepath.ToSlash(raw), "/")
		if p == "" {
			continue
		}
		anchored := strings.Contains(p, "/")
		if rest, found := strings.CutPrefix(p, "**/"); found {
			p, anchored = rest, false
		}
		if p == "" {
			continue
		}
		// Reject a malformed pattern up front rather than having every
		// path silently fail to match it.
		if _, err := path.Match(p, "probe"); err != nil {
			return excluder{}, fmt.Errorf("invalid exclude_path pattern %q: %w", raw, err)
		}
		e.patterns = append(e.patterns, excludePattern{pattern: p, anchored: anchored})
	}
	return e, nil
}

func (e excluder) match(relSlash string) bool {
	for _, p := range e.patterns {
		if p.anchored {
			if ok, _ := path.Match(p.pattern, relSlash); ok {
				return true
			}
			continue
		}
		// Unanchored: the pattern may match the path's tail at any depth,
		// so try every suffix. A single-segment pattern reduces to matching
		// the base name, which is the common case.
		segs := strings.Split(relSlash, "/")
		for i := range segs {
			if ok, _ := path.Match(p.pattern, strings.Join(segs[i:], "/")); ok {
				return true
			}
		}
	}
	return false
}

// Classification is the result of comparing localDir against a staged copy
// of the phone's tree.
type Classification struct {
	// LocalOnly: present locally, absent on the phone — candidates to send.
	LocalOnly []string
	// PhoneOnly: present on the phone, absent locally — candidates to copy in.
	PhoneOnly []string
	// Differ: present on both sides with a relevant difference — needs merge.
	Differ []string
	// Identical: present on both sides, byte-identical or differing only in
	// ways ignorePatterns says don't matter.
	Identical []string
	// Unreadable: a path that exists but couldn't be read for comparison —
	// reported rather than silently treated as unchanged, which is what the
	// bash tool did until it was fixed there too.
	Unreadable []string
	// LocalSymlinks: every local symlink found, resolvable or not — the
	// guardrail set. A path in here must never be written to by a
	// phone-to-local operation (pull's copy-in, sync's merge or copy-in),
	// regardless of what bucket above it also appears in.
	LocalSymlinks []string
	// ProtectedSymlinkOnPhone: present on the phone, but the local path is
	// currently a symlink — reported so you know it's there, never treated
	// as PhoneOnly (which would otherwise offer to copy it in and silently
	// replace the symlink with a plain file).
	ProtectedSymlinkOnPhone []string
	// Binary: paths present on both sides whose content is not text. They
	// are listed in Differ too when they differ, but they must never reach
	// a line merge — a union of two PNGs is not a PNG. The symlink branch
	// already reasoned this way about attachments; this extends the same
	// reasoning to a binary that happens not to be behind a symlink, which
	// in an Obsidian vault means every embedded image, every font, and
	// every PDF.
	Binary []string
	// BrokenSymlinks: local symlinks whose target could not be resolved,
	// reported only when this run asked for symlinks to be treated as
	// content (--with-symlinks). Without this they were skipped in silence,
	// so a vault whose attachment target had moved gave no hint that
	// anything had been left out.
	BrokenSymlinks []string
	// Walked is every path either side's walk saw, in any role at all —
	// files, symlinks, unreadable entries, both sides merged. It is the
	// authoritative "this path still exists somewhere" set, which is what
	// snapshot pruning needs: assembling that from the buckets above means
	// silently dropping whichever bucket gets added next, and a symlink
	// (which never enters the comparable-file set on a plain run) would
	// have its baseline pruned by the first run made without
	// --with-symlinks.
	Walked []string
}

// Classify compares localDir against stagedPhoneDir (a local, already
// pulled-down copy of the relevant phone subtree). includeSymlinks controls
// whether a local symlink that resolves to a regular file is treated as
// syncable content at all — see WalkFiles. Regardless of that flag, any
// local symlink is protected from ever appearing as a phone-to-local target
// (see LocalSymlinks / ProtectedSymlinkOnPhone above).
func Classify(localDir, stagedPhoneDir string, ignorePatterns, excludePaths []string, includeSymlinks bool) (Classification, error) {
	localFiles, localSymlinks, brokenSymlinks, err := WalkFiles(localDir, excludePaths, includeSymlinks)
	if err != nil {
		return Classification{}, err
	}
	// The staged phone tree is always plain regular files (StageIn writes
	// them with os.WriteFile), so symlink detection there is moot.
	phoneFiles, _, _, err := WalkFiles(stagedPhoneDir, excludePaths, false)
	if err != nil {
		return Classification{}, err
	}

	var c Classification
	// A file that exists on one side only still has to be readable to be
	// transferred. Checking it here rather than only on the both-sides
	// branch is what keeps a preview honest: an unreadable local note used
	// to be announced as "would copy to phone" and then fail at commit,
	// which is the one place a dry run is supposed to be trustworthy.
	for rel := range localFiles {
		if _, ok := phoneFiles[rel]; ok {
			continue
		}
		if !isReadable(filepath.Join(localDir, filepath.FromSlash(rel))) {
			c.Unreadable = append(c.Unreadable, rel)
			continue
		}
		c.LocalOnly = append(c.LocalOnly, rel)
	}
	for rel := range phoneFiles {
		if _, ok := localFiles[rel]; ok {
			continue
		}
		if _, guarded := localSymlinks[rel]; guarded {
			c.ProtectedSymlinkOnPhone = append(c.ProtectedSymlinkOnPhone, rel)
			continue
		}
		if !isReadable(filepath.Join(stagedPhoneDir, filepath.FromSlash(rel))) {
			c.Unreadable = append(c.Unreadable, rel)
			continue
		}
		c.PhoneOnly = append(c.PhoneOnly, rel)
	}
	for rel := range localFiles {
		if _, ok := phoneFiles[rel]; !ok {
			continue
		}
		localPath := filepath.Join(localDir, filepath.FromSlash(rel))
		phonePath := filepath.Join(stagedPhoneDir, filepath.FromSlash(rel))
		if !isReadable(localPath) || !isReadable(phonePath) {
			c.Unreadable = append(c.Unreadable, rel)
			continue
		}

		binary, err := relevance.IsBinary(localPath)
		if err != nil {
			return Classification{}, fmt.Errorf("inspecting %s: %w", rel, err)
		}
		if binary {
			c.Binary = append(c.Binary, rel)
		}

		var relevant bool
		if _, isSymlinkContent := localSymlinks[rel]; isSymlinkContent {
			// Symlinked content (attachments, binaries) isn't text a diff
			// hunk can usefully judge "relevant" on — compare raw bytes
			// instead of routing through the dateUpdated-aware line filter.
			identical, err := filesEqual(localPath, phonePath)
			if err != nil {
				return Classification{}, fmt.Errorf("comparing %s: %w", rel, err)
			}
			relevant = !identical
		} else {
			relevant, err = relevance.Relevant(localPath, phonePath, ignorePatterns)
			if err != nil {
				return Classification{}, fmt.Errorf("comparing %s: %w", rel, err)
			}
		}
		if relevant {
			c.Differ = append(c.Differ, rel)
		} else {
			c.Identical = append(c.Identical, rel)
		}
	}

	for rel := range localSymlinks {
		c.LocalSymlinks = append(c.LocalSymlinks, rel)
	}
	for rel := range brokenSymlinks {
		c.BrokenSymlinks = append(c.BrokenSymlinks, rel)
	}

	// Walked comes straight from the raw walk sets rather than from the
	// buckets above, so it stays complete no matter how the classification
	// is later subdivided.
	walked := make(map[string]struct{}, len(localFiles)+len(phoneFiles)+len(localSymlinks))
	for _, set := range []map[string]struct{}{localFiles, phoneFiles, localSymlinks} {
		for rel := range set {
			walked[rel] = struct{}{}
		}
	}
	for rel := range walked {
		c.Walked = append(c.Walked, rel)
	}

	for _, s := range [][]string{c.LocalOnly, c.PhoneOnly, c.Differ, c.Identical, c.Unreadable, c.LocalSymlinks, c.ProtectedSymlinkOnPhone, c.BrokenSymlinks, c.Binary, c.Walked} {
		sort.Strings(s)
	}
	return c, nil
}

func filesEqual(a, b string) (bool, error) {
	da, err := os.ReadFile(a)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", a, err)
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", b, err)
	}
	return bytes.Equal(da, db), nil
}

func isReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// Lock acquires a non-blocking exclusive advisory lock on lockPath, the same
// role `flock -n 9 ... 9>"$lockfile"` played in the bash tool — a
// double-launch, which a desktop panel button makes easy, must not run two
// syncs against the same trees concurrently. Call the returned release
// func to unlock; the file descriptor is closed either way.
func Lock(lockPath string) (release func() error, err error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if err == unix.EWOULDBLOCK {
			return nil, fmt.Errorf("another logSync instance is running (locked: %s)", lockPath)
		}
		return nil, fmt.Errorf("flock %s: %w", lockPath, err)
	}
	return func() error {
		defer f.Close()
		return unix.Flock(int(f.Fd()), unix.LOCK_UN)
	}, nil
}

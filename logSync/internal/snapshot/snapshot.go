// Package snapshot gives merge.Into a real ancestor for paths a git
// tracking repo can't provide one for. A merge scope is often deliberately
// gitignored — it is the noisiest part of a vault, which is why it is the
// part worth merging — and merge.Ancestor reports "not found" for every
// path in it, forcing every merge there back to the empty-base union. What
// that costs is concrete: a union against an empty base re-emits shared
// content whenever diff3 cannot align the two sides, and for an edited
// note it cannot align them, so repeats accumulate — an edit made
// independently on both devices to the same entry gets torn apart and
// partially duplicated instead of merged as two coherent, non-overlapping
// changes.
//
// The fix needs no new git history and no new privacy surface: after every
// path a sync round successfully re-stamps, local and phone are
// byte-identical for it (that convergence is sync's own postcondition).
// That converged content is a valid 3-way-merge ancestor for the *next*
// round — so this package just remembers it, keyed by relative path, and
// merge.Into gets real block-preserving diff3 behavior (each side's
// contiguous change lands as itself, not interleaved) instead of the
// degenerate one-hunk-per-line behavior an empty base produces.
package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/atomicfile"
)

// Locate returns where the snapshot store lives and whether it is there
// yet, without creating anything. Reads and reports go through this: an
// inspection command that created the store as a side effect would make
// "the store does not exist" an unobservable state.
func Locate() (dir string, exists bool, err error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir = filepath.Join(base, "logSync", "snapshot")
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dir, false, nil
		}
		return "", false, fmt.Errorf("stat snapshot dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("snapshot path %s exists but is not a directory", dir)
	}
	return dir, true, nil
}

// Dir returns the root snapshot directory, creating it if needed.
// $XDG_STATE_HOME/logSync/snapshot, falling back to
// ~/.local/state/logSync/snapshot — this is logSync's own bookkeeping,
// not vault content, so it belongs in state, not the config or vault trees.
//
// Note that the store holds a full copy of every note it has a baseline
// for. That is a second place vault content lives on this machine, outside
// the vault and outside any repo, which is worth knowing if some of it is
// sensitive.
func Dir() (string, error) {
	dir, _, err := Locate()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create snapshot dir %s: %w", dir, err)
	}
	return dir, nil
}

// Path returns where absLocalPath's snapshot lives inside the store. The
// key is the file's own absolute local path, mirrored under Dir() — not the
// path relative to whichever directory a given command happens to be
// syncing.
//
// Relative keys were wrong in two ways at once. Two different vaults with a
// note at the same relative path shared one snapshot and silently
// overwrote each other's ancestors. And the same file reached at two
// scopes got two different keys: `notesync` runs at vault scope and would
// record "journal/entry.md", while `sync` runs at merge scope and looks
// up "entry.md" — so a seeded device could never prime the merge scope's
// ancestors, and every note met its first two-sided merge with no base at
// all. Keying on the absolute path collapses both cases: one file, one
// snapshot, whichever command gets there first.
func Path(absLocalPath string) (string, error) {
	if !filepath.IsAbs(absLocalPath) {
		return "", fmt.Errorf("snapshot key must be an absolute local path, got %q", absLocalPath)
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(absLocalPath)
	return filepath.Join(dir, strings.TrimPrefix(clean, string(filepath.Separator))), nil
}

// Read returns the last-known-converged content for the file at
// absLocalPath, if any. found is false (with a nil error) when nothing has
// been recorded yet — the caller should fall back to an empty-base union
// merge in that case, exactly as when a git ancestor is unavailable.
func Read(absLocalPath string) (content []byte, found bool, err error) {
	key, err := Path(absLocalPath)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(key)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read snapshot for %s: %w", absLocalPath, err)
	}
	return data, true, nil
}

// Write records content as the new converged baseline for the file at
// absLocalPath, for the next merge round to use as an ancestor. Call this
// only after the content has actually been confirmed identical on both
// sides (e.g. after a successful StageOut) — recording a baseline that was
// never really reached would poison the next merge's 3-way comparison.
func Write(absLocalPath string, content []byte) error {
	key, err := Path(absLocalPath)
	if err != nil {
		return err
	}
	return atomicfile.Write(key, content)
}

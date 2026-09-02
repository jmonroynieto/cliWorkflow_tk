package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// State says what a stored baseline is still good for. The distinction that
// matters is Stale vs Orphaned: one note being gone from a directory that is
// still there means the note was deleted, and its baseline is dead weight. A
// whole directory being gone means the vault moved, or its mount is not up —
// and deleting every baseline under it because a disk was not mounted is the
// one mistake this package must not make on its own.
type State int

const (
	// Live: the file this baseline describes is still there.
	Live State = iota
	// Stale: the file is gone but its directory is not, so it was deleted or
	// renamed and the baseline can go.
	Stale
	// Orphaned: the containing directory itself is gone. Could be a deleted
	// folder, a moved vault, or an unmounted one — indistinguishable from
	// here, so these are only ever removed when asked for explicitly.
	Orphaned
)

func (s State) String() string {
	switch s {
	case Live:
		return "live"
	case Stale:
		return "stale"
	case Orphaned:
		return "orphaned"
	}
	return "unknown"
}

// Entry is one stored baseline: the local file it describes, where it sits
// in the store, and how much room it takes.
type Entry struct {
	LocalPath string
	StorePath string
	Size      int64
	State     State
}

// List walks the store and returns every baseline in it, newest layout or
// old. Passing a non-empty root restricts the walk to baselines for files
// under that local directory. A store that does not exist yet lists as
// empty rather than as an error.
//
// Baselines written by an older build used a key relative to whichever
// directory was being synced rather than the file's absolute path. Those
// map to local paths that were never real ("journal.md" becomes
// "/journal.md"), so they surface here as Stale or Orphaned and get cleaned
// up by the same prune that handles deleted notes — there is no separate
// migration to run.
func List(root string) ([]Entry, error) {
	dir, exists, err := Locate()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	walkRoot := dir
	if root != "" {
		if !filepath.IsAbs(root) {
			return nil, fmt.Errorf("list root must be an absolute path, got %q", root)
		}
		walkRoot = filepath.Join(dir, strings.TrimPrefix(filepath.Clean(root), string(filepath.Separator)))
		if _, err := os.Stat(walkRoot); os.IsNotExist(err) {
			return nil, nil
		}
	}

	var out []Entry
	err = filepath.WalkDir(walkRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		local := string(filepath.Separator) + rel
		out = append(out, Entry{
			LocalPath: local,
			StorePath: p,
			Size:      info.Size(),
			State:     stateOf(local),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking snapshot store: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LocalPath < out[j].LocalPath })
	return out, nil
}

func stateOf(localPath string) State {
	if _, err := os.Lstat(localPath); err == nil {
		return Live
	}
	if info, err := os.Stat(filepath.Dir(localPath)); err == nil && info.IsDir() {
		return Stale
	}
	return Orphaned
}

// Remove deletes one baseline and any directories it leaves empty, stopping
// at the store root so the store itself survives being emptied.
func Remove(e Entry) error {
	dir, exists, err := Locate()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := os.Remove(e.StorePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove snapshot %s: %w", e.StorePath, err)
	}
	// An emptied directory left behind would accumulate exactly as fast as
	// the files did.
	for p := filepath.Dir(e.StorePath); strings.HasPrefix(p, dir+string(filepath.Separator)); p = filepath.Dir(p) {
		if err := os.Remove(p); err != nil {
			// Not empty, or gone already — either way there is nothing
			// further up to tidy.
			return nil
		}
	}
	return nil
}

// PruneUnder removes the baselines under root whose files a sync round just
// established are gone from both sides, and returns what it removed. keep
// holds every absolute local path that round saw in any role — including
// symlinks, unreadable files, and paths whose phone-side state could not be
// determined, none of which are "gone" just because they were not merged.
//
// Orphaned baselines are never touched here: a run that cannot see its own
// vault has no business deciding that vault's history is garbage.
func PruneUnder(root string, keep map[string]struct{}) (removed []Entry, err error) {
	dead, err := StaleUnder(root, keep)
	if err != nil {
		return nil, err
	}
	for _, e := range dead {
		if err := Remove(e); err != nil {
			return removed, err
		}
		removed = append(removed, e)
	}
	return removed, nil
}

// StaleUnder is PruneUnder's decision half, split out so a preview can show
// exactly what a committed run would drop without dropping it.
func StaleUnder(root string, keep map[string]struct{}) (dead []Entry, err error) {
	// Anchor the "is this a deletion or a disappeared disk?" question at
	// the sync root rather than at each file's own parent. Deleting a
	// folder in Obsidian is an ordinary thing to do, and the per-parent
	// rule called every baseline under it orphaned — un-prunable without a
	// human, so a vault that gets reorganised accumulates them forever. If
	// the root itself is there, the vault is mounted and present, and
	// anything missing below it was genuinely deleted.
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		// The root is gone: this is the unmounted-vault case, and nothing
		// under it can be judged. The global `snapshot prune` command,
		// which has no root to anchor on, is where these get settled.
		return nil, nil
	}

	entries, err := List(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.State == Live {
			continue
		}
		if _, held := keep[e.LocalPath]; held {
			continue
		}
		dead = append(dead, e)
	}
	return dead, nil
}

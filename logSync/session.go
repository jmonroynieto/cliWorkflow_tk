package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/electricbubble/gadb"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/adbx"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/snapshot"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
)

// sessionScope selects which of Config's two directory pairs a session
// operates on — the narrow, merge-capable scope (sync/push/pull)
// or the whole-vault one-way scope (notesync). See Config's field comments.
type sessionScope int

const (
	scopeMerge sessionScope = iota
	scopeVault
)

// session bundles what every one of sync/push/pull/notesync needs: a
// validated config, the single-instance lock held, a connected device, and
// a local staging copy of the phone's tree pulled down once up front (the
// same "stage the whole subtree locally in one pass, then do every
// comparison on disk" shape the bash tool used, but over the adb sync
// protocol directly instead of a remote `tar`).
type session struct {
	cfg       *Config
	localDir  string
	remoteDir string
	dev       gadb.Device
	stageDir  string
	// stageFailed holds the paths the device listed but would not hand over.
	// Their phone-side state is unknown, not absent — the distinction
	// matters, because a file missing from the staged tree is otherwise
	// indistinguishable from one that was deleted on the phone, and gets
	// treated as a one-sided path or has its merge baseline pruned.
	stageFailed map[string]struct{}
	release     func() error
}

func lockPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "logSync.lock")
}

func openSession(configFlag string, scope sessionScope) (*session, error) {
	cfg, err := loadConfig(configFlag)
	if err != nil {
		return nil, err
	}

	var localDir, remoteDir string
	switch scope {
	case scopeMerge:
		if err := cfg.validateForSync(); err != nil {
			return nil, err
		}
		localDir, remoteDir = cfg.LocalDir, cfg.RemoteDir
	case scopeVault:
		if err := cfg.validateForNotesync(); err != nil {
			return nil, err
		}
		localDir, remoteDir = cfg.VaultLocalDir, cfg.VaultRemoteDir
	default:
		return nil, fmt.Errorf("internal error: unknown session scope %d", scope)
	}

	release, err := synctree.Lock(lockPath())
	if err != nil {
		return nil, err
	}

	client, err := adbx.Connect()
	if err != nil {
		release()
		return nil, err
	}
	dev, err := adbx.SelectDevice(client, cfg.DeviceSerial)
	if err != nil {
		release()
		return nil, err
	}

	stageDir, err := os.MkdirTemp("", "logSync-stage-*")
	if err != nil {
		release()
		return nil, fmt.Errorf("create staging dir: %w", err)
	}
	staged, err := adbx.StageIn(dev, remoteDir, stageDir, localDir, nil)
	if err != nil {
		os.RemoveAll(stageDir)
		release()
		return nil, fmt.Errorf("staging phone tree: %w", err)
	}
	stageFailed := make(map[string]struct{}, len(staged.Failed))
	for rel, ferr := range staged.Failed {
		fmt.Fprintf(os.Stderr, "logSync: warning: could not stage %s from phone, skipping: %v\n", rel, ferr)
		stageFailed[rel] = struct{}{}
	}
	if staged.Shortcut > 0 {
		fmt.Printf("logSync: staged %d file(s), %d already matched local and were not downloaded\n", len(staged.Succeeded), staged.Shortcut)
	}

	return &session{cfg: cfg, localDir: localDir, remoteDir: remoteDir, dev: dev, stageDir: stageDir, stageFailed: stageFailed, release: func() error {
		os.RemoveAll(stageDir)
		return release()
	}}, nil
}

// abs turns a path relative to this session's local directory into the
// absolute one, which is also its key in the snapshot store.
func (s *session) abs(rel string) string {
	return filepath.Join(s.localDir, filepath.FromSlash(rel))
}

// report prints the classification's advisory buckets — the things that
// happened to a path that are worth knowing about but are not themselves
// transfers. It also drops from LocalOnly any path whose absence from the
// staged tree only means the download failed, so a transient device error
// is never read as "deleted on the phone".
func (s *session) report(c synctree.Classification) synctree.Classification {
	for _, rel := range c.Unreadable {
		fmt.Printf("unreadable, skipped: %s\n", rel)
	}
	for _, rel := range c.BrokenSymlinks {
		fmt.Printf("symlink target missing, skipped: %s\n", rel)
	}
	for _, rel := range c.ProtectedSymlinkOnPhone {
		fmt.Printf("protected (local is a symlink, phone copy ignored): %s\n", rel)
	}
	if len(s.stageFailed) == 0 {
		return c
	}
	kept := c.LocalOnly[:0:0]
	for _, rel := range c.LocalOnly {
		if _, unknown := s.stageFailed[rel]; unknown {
			fmt.Printf("phone state unknown (staging failed), leaving alone: %s\n", rel)
			continue
		}
		kept = append(kept, rel)
	}
	c.LocalOnly = kept
	return c
}

// pruneSnapshots drops the merge baselines for files this round found on
// neither side. Without it the store grows forever: every note ever
// renamed or deleted leaves a full copy of its last converged content
// behind, and nothing ever revisits it.
//
// keep is built from Walked — every path either walk saw in any role — plus
// the paths whose phone-side state could not be determined. Baselines
// outside this session's own directory, and those whose containing
// directory has itself gone, are left alone; see snapshot.PruneUnder.
func (s *session) pruneSnapshots(c synctree.Classification, commit bool) {
	keep := make(map[string]struct{}, len(c.Walked)+len(s.stageFailed))
	for _, rel := range c.Walked {
		keep[s.abs(rel)] = struct{}{}
	}
	for rel := range s.stageFailed {
		keep[s.abs(rel)] = struct{}{}
	}
	if !commit {
		dead, err := snapshot.StaleUnder(s.localDir, keep)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logSync: warning: could not check merge baselines: %v\n", err)
			return
		}
		for _, e := range dead {
			fmt.Printf("would drop merge baseline (gone from both sides): %s\n", e.LocalPath)
		}
		return
	}
	removed, err := snapshot.PruneUnder(s.localDir, keep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logSync: warning: could not prune merge baselines: %v\n", err)
		return
	}
	for _, e := range removed {
		fmt.Printf("dropped merge baseline (gone from both sides): %s\n", e.LocalPath)
	}
}

func (s *session) close() {
	if err := s.release(); err != nil {
		fmt.Fprintf(os.Stderr, "logSync: warning: %v\n", err)
	}
}

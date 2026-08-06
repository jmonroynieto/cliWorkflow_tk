package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/id"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
)

// Workspace is a disposable scratch directory tracked by talaria.
type Workspace struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	Pinned      bool      `json:"pinned,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Notes       []Note    `json:"notes,omitempty"`
}

// Note is a free-form annotation on a workspace.
type Note struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// fileData is the on-disk JSON document.
type fileData struct {
	Version    int         `json:"version"`
	NextNoteID int64       `json:"next_note_id"`
	Workspaces []Workspace `json:"workspaces"`
}

// Store is a JSON-file workspace registry.
//
// Concurrency: Open acquires an exclusive flock on a sidecar lock file
// (workspaces.json.lock) and holds it until Close. That serializes every
// reader/writer — CLI processes today, and the future daemon as sole writer
// later — so the read-modify-write cycle cannot interleave.
//
// Writes themselves are also atomic (temp file + rename).
type Store struct {
	path     string
	lockPath string
	lockFile *os.File
	data     fileData
}

// Open locks the registry (blocking) and loads it into memory.
// The exclusive lock is held until Close.
func Open(path string) (*Store, error) {
	return open(context.Background(), path, true)
}

// OpenContext is like Open but respects ctx when waiting for the lock.
// Prefer this for the CLI local fallback so a live daemon does not hang the process forever.
func OpenContext(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, false)
}

func open(ctx context.Context, path string, blocking bool) (*Store, error) {
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}

	lockPath := path + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}

	if blocking {
		if err := flockExclusive(lf); err != nil {
			_ = lf.Close()
			return nil, fmt.Errorf("lock registry: %w", err)
		}
	} else {
		if err := flockExclusiveContext(ctx, lf); err != nil {
			_ = lf.Close()
			return nil, fmt.Errorf("lock registry: %w", err)
		}
	}

	s := &Store{
		path:     path,
		lockPath: lockPath,
		lockFile: lf,
		data: fileData{
			Version:    1,
			NextNoteID: 1,
			Workspaces: []Workspace{},
		},
	}

	// Load only after the lock is held so we never observe a partial write
	// or a state another process is mid-update on.
	if err := s.load(); err != nil {
		_ = s.Close()
		return nil, err
	}

	// First run: materialize an empty registry under the lock.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := s.save(); err != nil {
			_ = s.Close()
			return nil, err
		}
	}
	return s, nil
}

// Close releases the exclusive lock and closes the lock file.
func (s *Store) Close() error {
	if s.lockFile == nil {
		return nil
	}
	errUnlock := flockUnlock(s.lockFile)
	errClose := s.lockFile.Close()
	s.lockFile = nil
	if errUnlock != nil {
		return fmt.Errorf("unlock registry: %w", errUnlock)
	}
	return errClose
}

// Path returns the registry file path.
func (s *Store) Path() string {
	return s.path
}

// LockPath returns the sidecar lock file path.
func (s *Store) LockPath() string {
	return s.lockPath
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = fileData{
				Version:    1,
				NextNoteID: 1,
				Workspaces: []Workspace{},
			}
			return nil
		}
		return fmt.Errorf("read registry: %w", err)
	}
	if len(raw) == 0 {
		s.data = fileData{
			Version:    1,
			NextNoteID: 1,
			Workspaces: []Workspace{},
		}
		return nil
	}

	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("parse registry %s: %w", s.path, err)
	}
	if data.Version == 0 {
		data.Version = 1
	}
	if data.NextNoteID < 1 {
		data.NextNoteID = 1
	}
	if data.Workspaces == nil {
		data.Workspaces = []Workspace{}
	}
	s.data = data
	return nil
}

func (s *Store) save() error {
	if s.lockFile == nil {
		return fmt.Errorf("save without lock: store is closed")
	}

	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "workspaces-*.json.tmp")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	// rename is atomic on the same filesystem; combined with the exclusive
	// flock this is the full durability story for a single-machine registry.
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("rename registry: %w", err)
	}
	cleanup = false
	return nil
}

func (s *Store) indexOf(workspaceID string) int {
	for i := range s.data.Workspaces {
		if s.data.Workspaces[i].ID == workspaceID {
			return i
		}
	}
	return -1
}

func (s *Store) get(workspaceID string) (*Workspace, error) {
	i := s.indexOf(workspaceID)
	if i < 0 {
		return nil, fmt.Errorf("workspace %q not found", workspaceID)
	}
	return &s.data.Workspaces[i], nil
}

// CreateOptions controls Create.
type CreateOptions struct {
	Description string
	// Durable creates under paths.DurableRoot and pins the workspace
	// so it survives both reboot and age GC by default.
	Durable bool
}

// Create makes a new workspace directory and registers it (ephemeral root).
func (s *Store) Create(description string) (*Workspace, error) {
	return s.CreateOpts(CreateOptions{Description: description})
}

// CreateOpts makes a new workspace under the ephemeral or durable root.
func (s *Store) CreateOpts(opts CreateOptions) (*Workspace, error) {
	root := paths.WorkspaceRoot()
	pin := false
	if opts.Durable {
		var err error
		root, err = paths.DurableRoot()
		if err != nil {
			return nil, fmt.Errorf("durable root: %w", err)
		}
		pin = true
	}
	if err := paths.EnsureDir(root); err != nil {
		return nil, fmt.Errorf("workspace root: %w", err)
	}

	var (
		wsID string
		dir  string
		err  error
	)
	for attempt := 0; attempt < 8; attempt++ {
		wsID, err = id.New(4)
		if err != nil {
			return nil, err
		}
		if s.indexOf(wsID) >= 0 {
			continue
		}
		dir = filepath.Join(root, wsID)
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			break
		}
		if attempt == 7 {
			return nil, fmt.Errorf("could not allocate unique workspace id")
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	ws := Workspace{
		ID:          wsID,
		Path:        dir,
		Description: opts.Description,
		CreatedAt:   now,
		LastUsed:    now,
		Pinned:      pin,
	}
	s.data.Workspaces = append(s.data.Workspaces, ws)
	if err := s.save(); err != nil {
		_ = os.RemoveAll(dir)
		s.data.Workspaces = s.data.Workspaces[:len(s.data.Workspaces)-1]
		return nil, err
	}
	return &ws, nil
}

// PerdureOptions controls Perdure.
type PerdureOptions struct {
	// Root overrides paths.DurableRoot when non-empty.
	Root string
	// NoPin leaves the pin flag unchanged (default is to pin).
	NoPin bool
}

// Perdure moves a workspace directory onto durable storage and updates
// the registry path. By default the workspace is pinned (survives GC).
// Idempotent when already at the durable destination.
//
// pin = survive gc; durable path = survive reboot. Perdure does both
// unless NoPin is set.
func (s *Store) Perdure(workspaceID string, opts PerdureOptions) (*Workspace, error) {
	ws, err := s.get(workspaceID)
	if err != nil {
		return nil, err
	}

	root := opts.Root
	if root == "" {
		root, err = paths.DurableRoot()
		if err != nil {
			return nil, fmt.Errorf("durable root: %w", err)
		}
	}
	if err := paths.EnsureDir(root); err != nil {
		return nil, fmt.Errorf("durable root: %w", err)
	}

	oldPath := ws.Path
	newPath := filepath.Join(root, ws.ID)

	if _, err := os.Stat(oldPath); err != nil {
		return nil, fmt.Errorf("workspace directory missing: %s (cannot perdure)", oldPath)
	}

	moved := false
	if filepath.Clean(oldPath) != filepath.Clean(newPath) {
		if _, err := os.Stat(newPath); err == nil {
			return nil, fmt.Errorf("destination already exists: %s", newPath)
		}
		if err := relocateDir(oldPath, newPath); err != nil {
			return nil, fmt.Errorf("move to durable storage: %w", err)
		}
		ws.Path = newPath
		moved = true
	}

	if !opts.NoPin {
		ws.Pinned = true
	}
	ws.LastUsed = time.Now().UTC().Truncate(time.Second)
	if err := s.save(); err != nil {
		if moved {
			_ = relocateDir(newPath, oldPath)
			ws.Path = oldPath
		}
		return nil, err
	}
	return s.Get(workspaceID)
}

// relocateDir moves src to dst (rename, or copy+remove on cross-device).
func relocateDir(src, dst string) error {
	if err := paths.EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device or other rename failure: copy then remove source.
	if err := copyDir(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("copied to %s but failed to remove %s: %w", dst, src, err)
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			// Skip special files; workspaces are normal trees.
			return nil
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := paths.EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Get returns a copy of a workspace by ID.
func (s *Store) Get(workspaceID string) (*Workspace, error) {
	ws, err := s.get(workspaceID)
	if err != nil {
		return nil, err
	}
	cp := *ws
	cp.Tags = slices.Clone(ws.Tags)
	cp.Notes = slices.Clone(ws.Notes)
	return &cp, nil
}

// List returns all workspaces, pinned first, then newest last_used.
func (s *Store) List() ([]Workspace, error) {
	out := make([]Workspace, len(s.data.Workspaces))
	for i, ws := range s.data.Workspaces {
		out[i] = ws
		out[i].Tags = slices.Clone(ws.Tags)
		out[i].Notes = slices.Clone(ws.Notes)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].LastUsed.After(out[j].LastUsed)
	})
	return out, nil
}

// Touch updates last_used to now.
func (s *Store) Touch(workspaceID string) error {
	return s.TouchAt(workspaceID, time.Now().UTC().Truncate(time.Second))
}

// TouchAt sets last_used to t (also used by tests).
func (s *Store) TouchAt(workspaceID string, t time.Time) error {
	ws, err := s.get(workspaceID)
	if err != nil {
		return err
	}
	ws.LastUsed = t.UTC().Truncate(time.Second)
	return s.save()
}

// SetPinned sets or clears the pin flag.
func (s *Store) SetPinned(workspaceID string, pinned bool) error {
	ws, err := s.get(workspaceID)
	if err != nil {
		return err
	}
	ws.Pinned = pinned
	return s.save()
}

// Delete removes the registry entry. Caller may still remove the directory.
func (s *Store) Delete(workspaceID string) error {
	i := s.indexOf(workspaceID)
	if i < 0 {
		return fmt.Errorf("workspace %q not found", workspaceID)
	}
	s.data.Workspaces = append(s.data.Workspaces[:i], s.data.Workspaces[i+1:]...)
	return s.save()
}

// Remove deletes metadata and the workspace directory on disk.
func (s *Store) Remove(workspaceID string) error {
	ws, err := s.get(workspaceID)
	if err != nil {
		return err
	}
	path := ws.Path
	if err := s.Delete(workspaceID); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove dir %s: %w", path, err)
	}
	return nil
}

// GC removes unpinned workspaces whose last_used is older than maxAge.
// If maxAge is 0, removes all unpinned workspaces.
func (s *Store) GC(maxAge time.Duration) ([]string, error) {
	var cutoff time.Time
	if maxAge > 0 {
		cutoff = time.Now().UTC().Add(-maxAge)
	}

	var (
		kept    []Workspace
		removed []string
		paths   []string
	)
	for _, ws := range s.data.Workspaces {
		drop := !ws.Pinned && (maxAge <= 0 || ws.LastUsed.Before(cutoff))
		if drop {
			removed = append(removed, ws.ID)
			paths = append(paths, ws.Path)
			continue
		}
		kept = append(kept, ws)
	}
	if len(removed) == 0 {
		return nil, nil
	}
	s.data.Workspaces = kept
	if err := s.save(); err != nil {
		return nil, err
	}
	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
	return removed, nil
}

// MaintainResult is the outcome of a maintenance pass (reconcile + optional GC).
type MaintainResult struct {
	// Missing are registry IDs removed because their directory is gone
	// (user deleted the workspace manually, outside talaria).
	Missing []string `json:"missing,omitempty"`
	// GC are IDs removed by age-based garbage collection.
	GC []string `json:"gc,omitempty"`
	// Orphans are directories under the workspace root that are not in the
	// registry (informational; not deleted).
	Orphans []string `json:"orphans,omitempty"`
	// Kept is the number of workspaces remaining after the pass.
	Kept int `json:"kept"`
}

// MaintainOptions controls a maintenance pass.
type MaintainOptions struct {
	// GCOlderThan runs age-based GC after reconcile. 0 skips age GC
	// (reconcile still runs). Use a positive duration for timer jobs (e.g. 7d).
	GCOlderThan time.Duration
	// SkipGC disables age GC even if GCOlderThan is set.
	SkipGC bool
	// SkipOrphanScan skips scanning the workspace root for untracked dirs.
	SkipOrphanScan bool
}

// Maintain reconciles registry vs filesystem, then optionally runs age GC.
//
// Status check: if the user removed a workspace directory by hand (rm -rf),
// the registry entry is dropped. That is the "is the world still true?" pass
// intended for hourly/oneshot timers.
func (s *Store) Maintain(opts MaintainOptions) (*MaintainResult, error) {
	res := &MaintainResult{}

	// --- reconcile: registry → disk -----------------------------------------
	var kept []Workspace
	for _, ws := range s.data.Workspaces {
		fi, err := os.Stat(ws.Path)
		if err != nil || !fi.IsDir() {
			res.Missing = append(res.Missing, ws.ID)
			continue
		}
		kept = append(kept, ws)
	}
	if len(res.Missing) > 0 {
		s.data.Workspaces = kept
		if err := s.save(); err != nil {
			return nil, err
		}
	}

	// --- age GC -------------------------------------------------------------
	if !opts.SkipGC && opts.GCOlderThan > 0 {
		removed, err := s.GC(opts.GCOlderThan)
		if err != nil {
			return res, err
		}
		res.GC = removed
	}

	// --- orphan scan: disk → registry (report only) -------------------------
	if !opts.SkipOrphanScan {
		orphans, err := s.scanOrphans()
		if err != nil {
			return res, err
		}
		res.Orphans = orphans
	}

	res.Kept = len(s.data.Workspaces)
	if res.Missing == nil {
		res.Missing = []string{}
	}
	if res.GC == nil {
		res.GC = []string{}
	}
	if res.Orphans == nil {
		res.Orphans = []string{}
	}
	return res, nil
}

func (s *Store) scanOrphans() ([]string, error) {
	known := make(map[string]struct{}, len(s.data.Workspaces))
	for _, ws := range s.data.Workspaces {
		known[ws.ID] = struct{}{}
		known[filepath.Base(ws.Path)] = struct{}{}
	}

	roots := []string{paths.WorkspaceRoot()}
	if d, err := paths.DurableRoot(); err == nil && d != "" && filepath.Clean(d) != filepath.Clean(roots[0]) {
		roots = append(roots, d)
	}

	var orphans []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("scan workspace root %s: %w", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if _, ok := known[name]; ok {
				continue
			}
			orphans = append(orphans, filepath.Join(root, name))
		}
	}
	sort.Strings(orphans)
	return orphans, nil
}

// --- tags & notes -----------------------------------------------------------

// AddTag attaches a tag name to a workspace.
func (s *Store) AddTag(workspaceID, name string) error {
	ws, err := s.get(workspaceID)
	if err != nil {
		return err
	}
	if slices.Contains(ws.Tags, name) {
		return nil
	}
	ws.Tags = append(ws.Tags, name)
	sort.Strings(ws.Tags)
	return s.save()
}

// RemoveTag detaches a tag from a workspace.
func (s *Store) RemoveTag(workspaceID, name string) error {
	ws, err := s.get(workspaceID)
	if err != nil {
		return err
	}
	i := slices.Index(ws.Tags, name)
	if i < 0 {
		return fmt.Errorf("tag %q not found on workspace %q", name, workspaceID)
	}
	ws.Tags = append(ws.Tags[:i], ws.Tags[i+1:]...)
	return s.save()
}

// TagsFor returns tag names for a workspace.
func (s *Store) TagsFor(workspaceID string) ([]string, error) {
	ws, err := s.get(workspaceID)
	if err != nil {
		return nil, err
	}
	return slices.Clone(ws.Tags), nil
}

// FindByTag returns workspaces that have the given tag.
func (s *Store) FindByTag(name string) ([]Workspace, error) {
	var out []Workspace
	for _, ws := range s.data.Workspaces {
		if slices.Contains(ws.Tags, name) {
			cp := ws
			cp.Tags = slices.Clone(ws.Tags)
			cp.Notes = slices.Clone(ws.Notes)
			out = append(out, cp)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastUsed.After(out[j].LastUsed)
	})
	return out, nil
}

// AddNote appends a note to a workspace.
func (s *Store) AddNote(workspaceID, body string) (*Note, error) {
	ws, err := s.get(workspaceID)
	if err != nil {
		return nil, err
	}
	n := Note{
		ID:        s.data.NextNoteID,
		Body:      body,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	s.data.NextNoteID++
	ws.Notes = append(ws.Notes, n)
	if err := s.save(); err != nil {
		return nil, err
	}
	return &n, nil
}

// NotesFor returns notes for a workspace, oldest first.
func (s *Store) NotesFor(workspaceID string) ([]Note, error) {
	ws, err := s.get(workspaceID)
	if err != nil {
		return nil, err
	}
	return slices.Clone(ws.Notes), nil
}

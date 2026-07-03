package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StateDirName is the subdirectory under XDG_STATE_HOME (or ~/.local/state).
const StateDirName = "gromula"

// Filenames
const (
	PathStateFile   = "pathstate.json"
	OperationsLogFile = "operations.jsonl"
)

// PathEntry represents one entry in the managed PATH with full provenance.
type PathEntry struct {
	Path      string    `json:"path"`
	Source    string    `json:"source"`
	Action    string    `json:"action"` // "prepend", "append", "remove", "system"
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
}

// PathState is the persisted state document.
type PathState struct {
	Version   int           `json:"version"`
	UpdatedAt time.Time     `json:"updated_at"`
	SessionID string        `json:"session_id,omitempty"`
	Entries   []PathEntry   `json:"entries"`
}

// getStateDir returns the full path to ~/.local/state/gromula (or XDG_STATE_HOME equivalent)
// and ensures the directory exists with 0700 permissions.
func getStateDir() (string, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(stateHome, StateDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create state directory %s: %w", dir, err)
	}
	return dir, nil
}

// statePath returns the full path to pathstate.json.
func statePath() (string, error) {
	dir, err := getStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, PathStateFile), nil
}

// logPath returns the full path to operations.jsonl.
func logPath() (string, error) {
	dir, err := getStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, OperationsLogFile), nil
}

// Load reads the current PathState from disk. If the file does not exist,
// it returns a fresh empty state (caller should seed from current PATH if desired).
func Load() (*PathState, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &PathState{
			Version:   1,
			UpdatedAt: time.Now(),
			Entries:   []PathEntry{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var st PathState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", p, err)
	}
	return &st, nil
}

// Save writes the PathState atomically (temp file + rename).
func Save(st *PathState) error {
	st.UpdatedAt = time.Now()

	p, err := statePath()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(p), "pathstate-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file for atomic save: %w", err)
	}
	tmpName := tmp.Name()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to encode state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to sync state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp state file: %w", err)
	}

	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to atomically replace state file: %w", err)
	}
	return nil
}

// AppendOperation logs a single operation to the JSONL audit log.
// This is append-only and safe for concurrent (though rare) use during shell init.
func AppendOperation(op any) error {
	lp, err := logPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(lp, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open operations log: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(op); err != nil {
		return fmt.Errorf("failed to write operation to log: %w", err)
	}
	return nil
}

// SeedFromEnv creates initial PathEntry list from the current $PATH environment variable.
// Used when there is no existing state file.
func SeedFromEnv() []PathEntry {
	current := os.Getenv("PATH")
	if current == "" {
		return []PathEntry{}
	}

	sep := string(filepath.ListSeparator)
	parts := strings.Split(current, sep)

	entries := make([]PathEntry, 0, len(parts))
	now := time.Now()
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		entries = append(entries, PathEntry{
			Path:      p,
			Source:    "system",
			Action:    "system",
			Timestamp: now,
		})
	}
	return entries
}

// dedupAndOrder takes a list of paths (new ones first or last) and existing entries,
// applies removes, then adds while preserving order and removing duplicates (first seen wins).
// It returns the final ordered unique list and updates sources/timestamps where appropriate.
func dedupAndOrder(existing []PathEntry, toAdd []PathEntry, toRemove map[string]bool, prepend bool) []PathEntry {
	seen := make(map[string]bool)
	result := make([]PathEntry, 0, len(existing)+len(toAdd))

	// First, handle prepends or appends for new items
	newItems := make([]PathEntry, 0, len(toAdd))
	for _, e := range toAdd {
		if toRemove[e.Path] {
			continue // removed takes precedence
		}
		if !seen[e.Path] {
			seen[e.Path] = true
			newItems = append(newItems, e)
		}
	}

	if prepend {
		result = append(result, newItems...)
	}

	// Then existing entries (skip removed and already seen)
	for _, e := range existing {
		if toRemove[e.Path] {
			continue
		}
		if !seen[e.Path] {
			seen[e.Path] = true
			result = append(result, e)
		}
	}

	if !prepend {
		result = append(result, newItems...)
	}

	return result
}

// ApplyAdd updates the state by adding the given paths (with source and session).
// If a path already exists, it is treated as an "overwrite" (removed + re-inserted
// at the new position with Action="overwrite"). This allows repositioning paths.
func ApplyAdd(st *PathState, paths []string, source, sessionID string, prepend bool) (string, error) {
	now := time.Now()

	// Build a quick lookup of existing paths
	existingPaths := make(map[string]bool, len(st.Entries))
	for _, e := range st.Entries {
		existingPaths[e.Path] = true
	}

	toAdd := make([]PathEntry, 0, len(paths))
	pathsToOverwrite := make(map[string]bool)

	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned := filepath.Clean(p)

		action := "append"
		if prepend {
			action = "prepend"
		}

		if existingPaths[cleaned] {
			action = "overwrite"
			pathsToOverwrite[cleaned] = true
		}

		toAdd = append(toAdd, PathEntry{
			Path:      cleaned,
			Source:    source,
			Action:    action,
			Timestamp: now,
			SessionID: sessionID,
		})
	}

	// Remove old entries that we are overwriting so they can be re-positioned
	filtered := make([]PathEntry, 0, len(st.Entries))
	for _, e := range st.Entries {
		if !pathsToOverwrite[e.Path] {
			filtered = append(filtered, e)
		}
	}
	st.Entries = filtered

	toRemove := make(map[string]bool)
	st.Entries = dedupAndOrder(st.Entries, toAdd, toRemove, prepend)
	st.SessionID = sessionID

	if err := Save(st); err != nil {
		return "", fmt.Errorf("failed to save state after add: %w", err)
	}

	// Log operations (now correctly shows "overwrite" when applicable)
	for _, e := range toAdd {
		_ = AppendOperation(map[string]any{
			"op":         "add",
			"path":       e.Path,
			"source":     e.Source,
			"action":     e.Action,
			"timestamp":  e.Timestamp,
			"session_id": e.SessionID,
		})
	}

	return BuildPathString(st.Entries), nil
}

// ApplyRemove updates the state by removing the given paths.
func ApplyRemove(st *PathState, paths []string, source, sessionID string) (string, error) {
	now := time.Now()
	toRemove := make(map[string]bool, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			toRemove[filepath.Clean(p)] = true
		}
	}

	// We still record remove operations even if path wasn't present
	for p := range toRemove {
		_ = AppendOperation(map[string]any{
			"op":        "remove",
			"path":      p,
			"source":    source,
			"action":    "remove",
			"timestamp": now,
			"session_id": sessionID,
		})
	}

	st.Entries = dedupAndOrder(st.Entries, nil, toRemove, false) // prepend irrelevant for remove
	st.SessionID = sessionID

	if err := Save(st); err != nil {
		return "", fmt.Errorf("failed to save state after remove: %w", err)
	}

	return BuildPathString(st.Entries), nil
}

// ApplyClean deduplicates the current environment PATH without modifying state
// (useful for quick cleanup). It returns the cleaned PATH string.
func ApplyClean() string {
	current := os.Getenv("PATH")
	if current == "" {
		return ""
	}

	sep := string(filepath.ListSeparator)
	parts := strings.Split(current, sep)

	seen := make(map[string]bool)
	var result []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		result = append(result, p)
	}

	return strings.Join(result, sep)
}

// BuildPathString turns the ordered entries into a PATH string.
// Exported so CLI and other packages can get a clean, bash-ready PATH.
func BuildPathString(entries []PathEntry) string {
	if len(entries) == 0 {
		return ""
	}
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.Path
	}
	return strings.Join(paths, string(filepath.ListSeparator))
}

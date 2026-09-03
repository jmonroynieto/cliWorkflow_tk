package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/relevance"
)

const (
	configDirName  = "logSync"
	configFileName = "config"
)

// Config holds the paths logSync operates on. There is deliberately no
// baked-in default local/remote path: a wrong default is worse than none
// here, because it would point a sync at a directory the user did not
// choose, and both sides of that mistake are file trees. Both must be set
// explicitly.
type Config struct {
	// LocalDir is the merge-capable scope for sync/push/pull. It should
	// point at the small, frequently-changing subtree you actually want
	// merged — a journal or log directory, say — rather than at a whole
	// vault: merging is per-file work, and the cost of getting it wrong on
	// a file nobody edits on two devices is all downside.
	LocalDir string
	// RemoteDir is LocalDir's counterpart on the device, somewhere under
	// /storage/emulated/0.
	RemoteDir string
	// VaultLocalDir/VaultRemoteDir are the whole-vault paths for notesync —
	// the one-way, no-merge, local-always-wins mode modeled on the bash
	// tool's `notesync-all`, which moved the whole vault and shared no merge
	// logic with its other modes. Optional: unset unless you use notesync.
	VaultLocalDir  string
	VaultRemoteDir string
	// TrackingRepo is the git worktree root that tracks LocalDir's contents,
	// used to source a real 3-way-merge ancestor and to disambiguate
	// deletions. Optional: files outside a tracked scope fall back to a
	// union-only merge and an interactive prompt on apparent deletions.
	TrackingRepo string
	// IgnoreLinePatterns are regexp patterns; a diff hunk whose changed lines
	// all match one of these is not "relevant" (generalizes the bash tool's
	// hardcoded `dateUpdated:` rule). Plain strings are treated as
	// substrings-become-regexp via regexp.QuoteMeta by the caller if desired;
	// stored here as raw regexp source.
	IgnoreLinePatterns []string
	// DefaultIgnorePatterns adds relevance.ObsidianDateUpdatedPattern on top
	// of whatever IgnoreLinePatterns holds. On unless `default_ignore_patterns
	// = false` says otherwise, because a config that sets no patterns at all
	// makes every note Obsidian has merely touched a merge candidate: the
	// editor rewrites `dateUpdated:` on save, so opening a note on the phone
	// is enough to make it differ. The bash tool applied this same rule
	// unconditionally and hardcoded; making it a default that can be turned
	// off keeps that behavior without pretending it is the only one anybody
	// could want.
	DefaultIgnorePatterns bool
	// ExcludePaths are LocalDir-relative, forward-slash paths that are never
	// considered part of the synced tree, even though they live alongside
	// vault content — repo-management files like .gitconfig or .githooks
	// that were never meant to leave this machine. A directory entry excludes
	// its whole subtree.
	ExcludePaths []string
	// DeviceSerial optionally pins a specific device when more than one is
	// attached. Empty means "the only attached device" (error if != 1).
	DeviceSerial string
}

// configPath resolves $XDG_CONFIG_HOME/logSync/config, falling back to
// ~/.config/logSync/config. The store already honours XDG_STATE_HOME and
// the lock honours XDG_RUNTIME_DIR; hardcoding ~/.config here left the tool
// inconsistent with itself, and gave anyone who had moved their config root
// no way to say so short of --config on every invocation.
func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("failed to get current user: %w", err)
		}
		base = filepath.Join(usr.HomeDir, ".config")
	}
	return filepath.Join(base, configDirName, configFileName), nil
}

// loadConfig reads the logSync config file. It does not invent defaults
// for LocalDir/RemoteDir — validateForSync (called by the commands that need
// them) is what enforces they're present.
func loadConfig(explicitPath string) (*Config, error) {
	path := explicitPath
	if path == "" {
		var err error
		path, err = configPath()
		if err != nil {
			return nil, err
		}
	}

	cfg := &Config{DefaultIgnorePatterns: true}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && explicitPath == "" {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config at %s: %w", path, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "local_dir":
			cfg.LocalDir = value
		case "remote_dir":
			cfg.RemoteDir = value
		case "vault_local_dir":
			cfg.VaultLocalDir = value
		case "vault_remote_dir":
			cfg.VaultRemoteDir = value
		case "tracking_repo":
			cfg.TrackingRepo = value
		case "device_serial":
			cfg.DeviceSerial = value
		case "ignore_line_pattern":
			cfg.IgnoreLinePatterns = append(cfg.IgnoreLinePatterns, value)
		case "exclude_path":
			cfg.ExcludePaths = append(cfg.ExcludePaths, value)
		case "default_ignore_patterns":
			cfg.DefaultIgnorePatterns = !isFalse(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", path, err)
	}

	return cfg, nil
}

// ignorePatterns is the full set of "this difference is noise" rules a
// comparison should apply: the configured ones, plus the Obsidian preset
// unless it has been turned off.
func (c *Config) ignorePatterns() []string {
	if !c.DefaultIgnorePatterns {
		return c.IgnoreLinePatterns
	}
	out := make([]string, 0, len(c.IgnoreLinePatterns)+1)
	out = append(out, relevance.ObsidianDateUpdatedPattern)
	return append(out, c.IgnoreLinePatterns...)
}

func isFalse(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false", "no", "off", "0":
		return true
	}
	return false
}

// validateForSync enforces the invariants sync/push/pull need but doctor
// does not (doctor should run against just a device, no local paths needed
// for the connectivity checks).
func (c *Config) validateForSync() error {
	if err := validateDirPair("local_dir", c.LocalDir, "remote_dir", c.RemoteDir); err != nil {
		return err
	}
	if c.TrackingRepo != "" && !filepath.IsAbs(c.TrackingRepo) {
		return fmt.Errorf("tracking_repo must be an absolute path, got %q", c.TrackingRepo)
	}
	return nil
}

// validateForNotesync enforces the invariants the one-way whole-vault
// notesync command needs.
func (c *Config) validateForNotesync() error {
	return validateDirPair("vault_local_dir", c.VaultLocalDir, "vault_remote_dir", c.VaultRemoteDir)
}

func validateDirPair(localKey, localVal, remoteKey, remoteVal string) error {
	if localVal == "" || remoteVal == "" {
		p, _ := configPath()
		return fmt.Errorf("%s and %s must both be set in %s (or via --config)", localKey, remoteKey, p)
	}
	if !filepath.IsAbs(localVal) {
		return fmt.Errorf("%s must be an absolute path, got %q", localKey, localVal)
	}
	if !strings.HasPrefix(remoteVal, "/") {
		return fmt.Errorf("%s must be an absolute device path, got %q", remoteKey, remoteVal)
	}
	info, err := os.Stat(localVal)
	if err != nil {
		return fmt.Errorf("%s %s: %w", localKey, localVal, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %s is not a directory", localKey, localVal)
	}
	return nil
}

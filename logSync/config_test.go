package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/relevance"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func hasPattern(patterns []string, want string) bool {
	for _, p := range patterns {
		if p == want {
			return true
		}
	}
	return false
}

// Obsidian rewrites `dateUpdated:` on save, so a config with no ignore
// patterns makes every note the phone has merely opened a merge candidate.
// The bash tool applied this rule unconditionally; keeping it as a default
// preserves that, and the opt-out keeps it from being a hidden rule.
func TestIgnorePatterns_ObsidianPresetIsOnByDefault(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, "local_dir=/v\nremote_dir=/sd/v\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !hasPattern(cfg.ignorePatterns(), relevance.ObsidianDateUpdatedPattern) {
		t.Errorf("expected the Obsidian preset by default, got %v", cfg.ignorePatterns())
	}
}

func TestIgnorePatterns_PresetCanBeTurnedOff(t *testing.T) {
	for _, off := range []string{"false", "no", "off", "0", "False"} {
		cfg, err := loadConfig(writeConfig(t, "default_ignore_patterns="+off+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.ignorePatterns()) != 0 {
			t.Errorf("%q should disable the preset, got %v", off, cfg.ignorePatterns())
		}
	}
}

func TestIgnorePatterns_ConfiguredPatternsSurviveAlongsideThePreset(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, "ignore_line_pattern=^[+-]lastSeen:\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.ignorePatterns()
	if !hasPattern(got, relevance.ObsidianDateUpdatedPattern) || !hasPattern(got, "^[+-]lastSeen:") {
		t.Errorf("expected both the preset and the configured pattern, got %v", got)
	}
}

// The preset has to actually do its job end to end: two copies of a note
// differing only in the timestamp Obsidian rewrote are not worth merging.
func TestIgnorePatterns_PresetMakesATimestampOnlyDiffIrrelevant(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, "local_dir=/v\nremote_dir=/sd/v\n"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	body := "---\ntitle: Tasks\ndateUpdated: %s\n---\n\n- [ ] one\n"
	if err := os.WriteFile(a, []byte(strings.Replace(body, "%s", "2026-08-26T08:00:00", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(strings.Replace(body, "%s", "2026-08-29T23:59:00", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	relevant, err := relevance.Relevant(a, b, cfg.ignorePatterns())
	if err != nil {
		t.Fatal(err)
	}
	if relevant {
		t.Error("a dateUpdated-only difference should not be worth merging")
	}
}

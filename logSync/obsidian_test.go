package main

import (
	"testing"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
)

func TestIsObsidianPath(t *testing.T) {
	for path, want := range map[string]bool{
		".obsidian/app.json":                 true,
		".obsidian/plugins/dataview/main.js": true,
		"TN/.obsidian/app.json":              true, // a vault nested in a vault
		"notes/.obsidian/x":                  true,
		"notes/obsidian/x":                   false,
		"notes/my.obsidian.md":               false,
		"attachments/diagram.png":            false,
		".obsidianish/app.json":              false,
	} {
		if got := isObsidianPath(path); got != want {
			t.Errorf("isObsidianPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// Off by default: the base configuration the phone has is enough, and the
// directory is both the noisiest and the least mergeable part of a vault.
func TestEffectiveExcludes_ConfigDirectoryIsExcludedUnlessAskedFor(t *testing.T) {
	cfg := &Config{ExcludePaths: []string{".githooks"}}

	off := effectiveExcludes(cfg, false)
	if len(off) != 2 || off[1] != obsidianDirName {
		t.Errorf("expected the config directory appended by default, got %v", off)
	}
	on := effectiveExcludes(cfg, true)
	if len(on) != 1 {
		t.Errorf("--with-obsidian should leave the configured exclusions alone, got %v", on)
	}
	// The caller's slice must not be modified in place.
	if len(cfg.ExcludePaths) != 1 {
		t.Errorf("config was mutated: %v", cfg.ExcludePaths)
	}
}

// Even when the directory is looked at, nothing in it reaches a bucket that
// gets written: it is a union merge of JSON that breaks a plugin's
// settings, so the differences are for a human to read.
func TestSplitObsidian_ConfigPathsNeverReachAnActionBucket(t *testing.T) {
	c := synctree.Classification{
		Differ:    []string{".obsidian/app.json", "Notes/a.md"},
		LocalOnly: []string{".obsidian/plugins/x/data.json", "Notes/b.md"},
		PhoneOnly: []string{"TN/.obsidian/hotkeys.json", "Notes/c.md"},
	}
	rest, differ, localOnly, phoneOnly := splitObsidian(c)

	if len(rest.Differ) != 1 || rest.Differ[0] != "Notes/a.md" {
		t.Errorf("Differ still holds config paths: %v", rest.Differ)
	}
	if len(rest.LocalOnly) != 1 || len(rest.PhoneOnly) != 1 {
		t.Errorf("one-sided buckets still hold config paths: %v / %v", rest.LocalOnly, rest.PhoneOnly)
	}
	if len(differ) != 1 || len(localOnly) != 1 || len(phoneOnly) != 1 {
		t.Errorf("config paths were dropped rather than reported: %v %v %v", differ, localOnly, phoneOnly)
	}
}

func TestBubbleNeedsObsidian(t *testing.T) {
	if !bubbleNeedsObsidian([]string{"Notes/a.md", ".obsidian/app.json"}) {
		t.Error("a named config path has to be staged even without the flag")
	}
	if bubbleNeedsObsidian([]string{"Notes/a.md"}) {
		t.Error("an ordinary path should not pull in the whole config directory")
	}
	if bubbleNeedsObsidian(nil) {
		t.Error("nothing requested, nothing needed")
	}
}

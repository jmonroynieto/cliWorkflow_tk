package main

import (
	"bytes"
	"strings"
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

// Always excluded: the base configuration the phone has is enough, and the
// directory is both the noisiest and the least mergeable part of a vault.
// The one exception is staging for a --bubble that names a path inside it.
func TestEffectiveExcludes_ConfigDirectoryIsExcludedUnlessStagedForBubble(t *testing.T) {
	cfg := &Config{ExcludePaths: []string{".githooks"}}

	off := effectiveExcludes(cfg, false)
	if len(off) != 2 || off[1] != obsidianDirName {
		t.Errorf("expected the config directory appended by default, got %v", off)
	}
	on := effectiveExcludes(cfg, true)
	if len(on) != 1 {
		t.Errorf("staging for --bubble should leave the configured exclusions alone, got %v", on)
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

// configsync prints a runnable command and touches nothing. The push
// destination is the vault root rather than the .obsidian path itself:
// `adb push a/.obsidian b/.obsidian` would nest a copy inside the existing
// directory, which is the mistake this command exists to spare the user.
func TestPrintConfigsync_PushesToVaultRootAndRunsNothing(t *testing.T) {
	cfg := &Config{
		VaultLocalDir:  "/home/u/vault",
		VaultRemoteDir: "/storage/emulated/0/vault",
		DeviceSerial:   "ABC123",
	}
	var buf bytes.Buffer
	if err := printConfigsync(&buf, cfg); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "adb -s ABC123 push /home/u/vault/.obsidian /storage/emulated/0/vault\n") {
		t.Errorf("push line missing or wrong:\n%s", out)
	}
	if !strings.Contains(out, "adb -s ABC123 pull /storage/emulated/0/vault/.obsidian /home/u/vault\n") {
		t.Errorf("pull line missing or wrong:\n%s", out)
	}
}

// An unpinned device must not get a -s flag invented for it.
func TestPrintConfigsync_NoSerialNoFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := printConfigsync(&buf, &Config{VaultLocalDir: "/a", VaultRemoteDir: "/b"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), " -s ") {
		t.Errorf("unpinned device should not print -s:\n%s", buf.String())
	}
}

// A vault path with a space must survive being pasted into a shell.
func TestShellArg(t *testing.T) {
	if got := shellArg("/home/u/My Vault"); got != "'/home/u/My Vault'" {
		t.Errorf("shellArg with a space = %q", got)
	}
	if got := shellArg("/home/u/vault"); got != "/home/u/vault" {
		t.Errorf("shellArg should leave a plain path alone, got %q", got)
	}
}

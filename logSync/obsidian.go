package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/atomicfile"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
	"github.com/urfave/cli/v3"
)

// obsidianDirName is the vault's own configuration directory: themes,
// hotkeys, plugin code and per-plugin settings.
const obsidianDirName = ".obsidian"

// bubbleFlag is the sanctioned way for a configuration change made on the
// phone to reach this machine: name the file, and only that file moves.
var bubbleFlag = &cli.StringSliceFlag{
	Name:  "bubble",
	Usage: "copy this path from the phone to here, overwriting the local copy (repeatable; the one way a " + obsidianDirName + "/ change travels)",
}

// isObsidianPath reports whether rel lies inside a vault configuration
// directory at any depth — a vault nested inside another vault has its own.
func isObsidianPath(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == obsidianDirName {
			return true
		}
	}
	return false
}

// effectiveExcludes is the configured exclusions plus the vault
// configuration directory, which no sync mode walks.
//
// The base configuration the phone already has is enough for the work that
// gets done there, and syncing it costs more than it returns. Obsidian
// rewrites several of those files every time it starts, so they would be
// merge candidates constantly. They are JSON, and a union merge of JSON
// without a baseline does not produce JSON — it produces duplicated keys
// and a file the plugin cannot load. The directory is also most of the
// vault by file count: plugin code, fonts, themes.
//
// stageObsidian is set only by a --bubble naming a path inside it, which
// has to be staged to be copied. Whole-directory moves are `configsync`'s
// business, and it hands the job to adb rather than doing it here.
func effectiveExcludes(cfg *Config, stageObsidian bool) []string {
	if stageObsidian {
		return cfg.ExcludePaths
	}
	out := make([]string, 0, len(cfg.ExcludePaths)+1)
	out = append(out, cfg.ExcludePaths...)
	return append(out, obsidianDirName)
}

// splitObsidian removes every vault-configuration path from the buckets a
// command acts on, and returns them separately for reporting. Nothing
// inside the configuration directory is ever merged, sent or copied in by
// an ordinary run: the differences are for a human to read and act on,
// with --bubble as the one narrow exception.
func splitObsidian(c synctree.Classification) (rest synctree.Classification, cfgDiffer, cfgLocalOnly, cfgPhoneOnly []string) {
	keep := func(in []string, out *[]string) []string {
		kept := in[:0:0]
		for _, rel := range in {
			if isObsidianPath(rel) {
				*out = append(*out, rel)
				continue
			}
			kept = append(kept, rel)
		}
		return kept
	}
	rest = c
	rest.Differ = keep(c.Differ, &cfgDiffer)
	rest.LocalOnly = keep(c.LocalOnly, &cfgLocalOnly)
	rest.PhoneOnly = keep(c.PhoneOnly, &cfgPhoneOnly)
	return rest, cfgDiffer, cfgLocalOnly, cfgPhoneOnly
}

// reportObsidian prints what the configuration directory looks like on both
// sides, with a real diff for anything that differs, so the decision can be
// made from the actual change rather than from a filename.
func reportObsidian(s *session, differ, localOnly, phoneOnly, bubbled []string) {
	// Anything just taken from the phone is settled; listing it as still
	// differing would be reporting the state this run started in.
	taken := make(map[string]struct{}, len(bubbled))
	for _, rel := range bubbled {
		taken[strings.Trim(filepath.ToSlash(rel), "/")] = struct{}{}
	}
	drop := func(in []string) []string {
		out := in[:0:0]
		for _, rel := range in {
			if _, done := taken[rel]; !done {
				out = append(out, rel)
			}
		}
		return out
	}
	differ, localOnly, phoneOnly = drop(differ), drop(localOnly), drop(phoneOnly)
	if len(differ)+len(localOnly)+len(phoneOnly) == 0 {
		return
	}
	fmt.Printf("\n%s/ — review only, nothing here is written by this run\n", obsidianDirName)
	for _, rel := range localOnly {
		fmt.Printf("  here only:  %s\n", rel)
	}
	for _, rel := range phoneOnly {
		fmt.Printf("  phone only: %s   (--bubble %q to take it)\n", rel, rel)
	}
	for _, rel := range differ {
		fmt.Printf("  differs:    %s   (--bubble %q to take the phone's)\n", rel, rel)
		fmt.Print(indent(configDiff(s, rel)))
	}
}

// configDiff renders the change as a unified diff, capped — a plugin's
// data.json can be megabytes, and the point is to see what moved.
func configDiff(s *session, rel string) string {
	const maxLines = 24
	local := filepath.Join(s.localDir, filepath.FromSlash(rel))
	phone := filepath.Join(s.stageDir, filepath.FromSlash(rel))
	out, err := exec.Command("diff", "-u", "--label", "here", "--label", "phone", local, phone).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() > 1 {
			return fmt.Sprintf("(could not diff: %v)\n", err)
		}
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("... %d more lines", len(lines)-maxLines))
	}
	return strings.Join(lines, "\n") + "\n"
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("    " + line + "\n")
	}
	return b.String()
}

// runBubble copies the named paths from the staged phone tree over the
// local ones. This is deliberately explicit and one-directional: a
// configuration change made on the phone is a thing you decided to keep,
// named by hand, not something a sync round infers.
func runBubble(s *session, rels []string, commit bool) error {
	for _, rel := range rels {
		rel = strings.Trim(filepath.ToSlash(rel), "/")
		src := filepath.Join(s.stageDir, filepath.FromSlash(rel))
		dst := filepath.Join(s.localDir, filepath.FromSlash(rel))
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("--bubble %s: not on the phone under %s: %w", rel, s.remoteDir, err)
		}
		if !commit {
			fmt.Printf("would bubble up (phone wins): %s\n", rel)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("--bubble %s: %w", rel, err)
		}
		if err := atomicfile.CopyPreservingTimestamp(src, dst); err != nil {
			return fmt.Errorf("--bubble %s: %w", rel, err)
		}
		fmt.Printf("bubbled up (phone wins): %s\n", rel)
		// Both sides now hold the phone's bytes at this path, which is a
		// real convergence and therefore a valid future merge base.
		if err := recordConvergence(dst); err != nil {
			fmt.Fprintf(os.Stderr, "logSync: warning: could not snapshot %s: %v\n", rel, err)
		}
	}
	return nil
}

// bubbleNeedsObsidian reports whether any requested path lives in the
// configuration directory, so a run can stage it even without the flag.
func bubbleNeedsObsidian(rels []string) bool {
	for _, rel := range rels {
		if isObsidianPath(strings.Trim(filepath.ToSlash(rel), "/")) {
			return true
		}
	}
	return false
}

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/adbx"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/atomicfile"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/merge"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/snapshot"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

var previewFlag = &cli.BoolFlag{
	Name:  "preview",
	Usage: "show what would happen without changing anything (default)",
}

var commitFlag = &cli.BoolFlag{
	Name:  "commit",
	Usage: "actually apply the changes; without this, preview mode runs",
}

// withSymlinksFlag is the opt-in, per-invocation "special mode" for
// attachment symlinks: off by default, so a local symlink is invisible to
// every comparison and the scope stays what it was meant to be — a small,
// frequently-changing text tree, not the whole vault including whatever
// binaries the notes happen to link to. On, push may dereference and send a
// resolvable symlink's target; pull/sync are still never allowed to write
// to a path that's currently a local symlink (see synctree.Classification.
// LocalSymlinks) — that guardrail holds regardless of this flag, so a
// stray phone-side copy from an earlier --with-symlinks push can never
// come back and overwrite the link on a later plain run.
var withSymlinksFlag = &cli.BoolFlag{
	Name:  "with-symlinks",
	Usage: "also treat local symlinks (e.g. attachments) as syncable content; push may send them, pull/sync never write over them",
}

// pruneFlag inverts the default guess for a phone-only path: instead of
// "not on local yet, copy it down" (sync) or "not local's concern, leave it"
// (push), it's "local doesn't have it, so it was deleted here — remove it
// from the phone too". No prompt, no git-tracked/untracked distinction the
// way splitOneSided draws it: this is specifically for scopes like LOGFILE
// that are deliberately untracked, where that distinction has no history to
// draw on and every phone-only file otherwise reads as "new".
//
// This is one-directional and can destroy content: a note created directly
// on the phone and never yet pulled down looks exactly like one deleted
// locally. It is opt-in per invocation for that reason.
var pruneFlag = &cli.BoolFlag{
	Name:  "prune",
	Usage: "phone-only files (missing locally) are assumed deleted and removed from the phone, instead of copied back",
}

// syncCommand is the bidirectional mode, which the bash tool confusingly
// called `pull` even though it wrote in both directions: merge differing
// files, copy files that exist on only one side, detect and ask about
// apparent deletions, then re-stamp only what actually changed back to the
// phone.
var syncCommand = &cli.Command{
	Name:  "sync",
	Usage: "bidirectional sync: merge differing files, copy missing ones both ways",
	Flags: []cli.Flag{previewFlag, commitFlag, withSymlinksFlag, withObsidianFlag, bubbleFlag, pruneFlag},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return runSync(cmd)
	},
}

func runSync(cmd *cli.Command) error {
	commit := cmd.Bool("commit")

	s, err := openSession(cmd.String("config"), scopeMerge)
	if err != nil {
		return err
	}
	defer s.close()

	bubble := cmd.StringSlice("bubble")
	// A named --bubble path inside the configuration directory has to be
	// staged to be copied, whether or not this run asked to look at the
	// rest of it.
	withObsidian := cmd.Bool("with-obsidian") || bubbleNeedsObsidian(bubble)

	c, err := synctree.Classify(s.localDir, s.stageDir, s.cfg.ignorePatterns(), effectiveExcludes(s.cfg, withObsidian), cmd.Bool("with-symlinks"))
	if err != nil {
		return err
	}
	c = s.report(c)
	c, cfgDiffer, cfgLocalOnly, cfgPhoneOnly := splitObsidian(c)
	s.pruneSnapshots(c, commit)

	if len(bubble) > 0 {
		if err := runBubble(s, bubble, commit); err != nil {
			return err
		}
	}
	// Both of these are "content a line merge has no business touching":
	// a symlinked attachment, and a plain file that simply is not text.
	// They take the same route — local wins, send it as it is.
	unmergeable := make(map[string]struct{}, len(c.LocalSymlinks)+len(c.Binary))
	for _, rel := range c.LocalSymlinks {
		unmergeable[rel] = struct{}{}
	}
	binary := make(map[string]struct{}, len(c.Binary))
	for _, rel := range c.Binary {
		unmergeable[rel] = struct{}{}
		binary[rel] = struct{}{}
	}

	// Classification first, decisions second. splitOneSided only sorts each
	// one-sided path into "copy it" or "a human has to say", and never
	// blocks: the whole plan is on screen before the first question is
	// asked, so the answer is given with the rest of the round in view.
	localOnlyToSend, localNeedsDecision, err := splitOneSided(s, c.LocalOnly)
	if err != nil {
		return err
	}
	prune := cmd.Bool("prune")
	var phoneOnlyToCopy, phoneNeedsDecision, toPrune []string
	if prune {
		toPrune = c.PhoneOnly
	} else {
		phoneOnlyToCopy, phoneNeedsDecision, err = splitOneSided(s, c.PhoneOnly)
		if err != nil {
			return err
		}
	}

	if len(c.Differ) == 0 && len(localOnlyToSend) == 0 && len(phoneOnlyToCopy) == 0 &&
		len(localNeedsDecision) == 0 && len(phoneNeedsDecision) == 0 && len(toPrune) == 0 {
		fmt.Println("logSync: already in sync (nothing to do)")
		reportObsidian(s, cfgDiffer, cfgLocalOnly, cfgPhoneOnly, bubble)
		return nil
	}
	defer reportObsidian(s, cfgDiffer, cfgLocalOnly, cfgPhoneOnly, bubble)

	// Under --commit every action announces itself as it happens, so
	// restating it up front as "would ..." just prints each path twice.
	// The pending decisions are the exception: those are worth seeing
	// before the first prompt appears.
	if !commit {
		for _, rel := range c.Differ {
			switch {
			case isIn(binary, rel):
				fmt.Printf("would send (binary, not merged, local wins): %s\n", rel)
			case isIn(unmergeable, rel):
				fmt.Printf("would send (symlink, local wins): %s\n", rel)
			default:
				fmt.Printf("would merge: %s\n", rel)
			}
		}
		for _, rel := range localOnlyToSend {
			fmt.Printf("would copy to phone: %s\n", rel)
		}
		for _, rel := range phoneOnlyToCopy {
			fmt.Printf("would copy to local: %s\n", rel)
		}
		for _, rel := range toPrune {
			fmt.Printf("would remove from phone (pruned, missing locally): %s\n", rel)
		}
	}
	for _, rel := range localNeedsDecision {
		fmt.Printf("needs decision (tracked, missing on phone): %s\n", rel)
	}
	for _, rel := range phoneNeedsDecision {
		fmt.Printf("needs decision (tracked, missing on local): %s\n", rel)
	}
	if !commit {
		fmt.Println("logSync: preview only, pass --commit to apply")
		return nil
	}

	// v1 never deletes: answering "yes, it was deleted" only means "don't
	// copy it back", so an unanswerable prompt can safely fall through to
	// copying — the conservative side either way.
	keptLocal, err := decideOneSided(s, localNeedsDecision, sideLocal)
	if err != nil {
		return err
	}
	localOnlyToSend = append(localOnlyToSend, keptLocal...)
	keptPhone, err := decideOneSided(s, phoneNeedsDecision, sidePhone)
	if err != nil {
		return err
	}
	phoneOnlyToCopy = append(phoneOnlyToCopy, keptPhone...)

	var toSend []string

	for _, rel := range c.Differ {
		if _, cannotMerge := unmergeable[rel]; cannotMerge {
			// Content there is no meaningful 3-way union of: a PDF, a PNG,
			// a font, or whatever an attachment symlink points at. Local is
			// authoritative for these paths by design, so re-send rather
			// than merge. Note what this costs — a change made to the same
			// binary on the phone is discarded here, and unlike a text
			// conflict there is no half-way result to inspect afterwards.
			if isIn(binary, rel) {
				fmt.Printf("sending (binary, not merged, local wins): %s\n", rel)
			} else {
				fmt.Printf("sending (symlink, local wins): %s\n", rel)
			}
			toSend = append(toSend, rel)
			continue
		}

		localPath := filepath.Join(s.localDir, filepath.FromSlash(rel))
		phonePath := filepath.Join(s.stageDir, filepath.FromSlash(rel))

		ancestor, hasAncestor, err := resolveAncestor(s.cfg.TrackingRepo, s.localDir, rel)
		if err != nil {
			return fmt.Errorf("looking up ancestor for %s: %w", rel, err)
		}

		res, err := merge.Into(localPath, ancestor, hasAncestor, phonePath, localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logSync: merge failed: %s: %v\n", rel, err)
			continue
		}
		fmt.Printf("merged: %s\n", rel)
		toSend = append(toSend, rel)
		_ = res
	}

	for _, rel := range phoneOnlyToCopy {
		src := filepath.Join(s.stageDir, filepath.FromSlash(rel))
		dst := filepath.Join(s.localDir, filepath.FromSlash(rel))
		if err := atomicfile.CopyPreservingTimestamp(src, dst); err != nil {
			return fmt.Errorf("copy to local %s: %w", rel, err)
		}
		fmt.Printf("copied to local: %s\n", rel)
		// Local now matches the phone at this path — record that convergence
		// so the next round has a real ancestor here too.
		if err := recordConvergence(dst); err != nil {
			fmt.Fprintf(os.Stderr, "logSync: warning: could not snapshot %s: %v\n", rel, err)
		}
	}
	toSend = append(toSend, localOnlyToSend...)

	if len(toSend) > 0 {
		res, err := adbx.StageOut(s.dev, s.localDir, s.remoteDir, toSend, func(rel string) {
			fmt.Printf("sent: %s\n", rel)
		})
		if err != nil {
			return err
		}
		for _, rel := range res.Succeeded {
			// Local and phone are now byte-identical at rel (StageOut just
			// sent exactly this content) — record it as the ancestor for the
			// next round's merge, whether or not this round involved one.
			localPath := filepath.Join(s.localDir, filepath.FromSlash(rel))
			if err := recordConvergence(localPath); err != nil {
				fmt.Fprintf(os.Stderr, "logSync: warning: could not snapshot %s: %v\n", rel, err)
			}
		}
		if len(res.Failed) > 0 {
			for rel, ferr := range res.Failed {
				fmt.Fprintf(os.Stderr, "logSync: failed to send %s: %v\n", rel, ferr)
			}
			return fmt.Errorf("sync: %d of %d files failed to send, %d sent", len(res.Failed), len(toSend), len(res.Succeeded))
		}
	}

	if len(toPrune) > 0 {
		res, err := adbx.DeleteBatch(s.dev, s.remoteDir, toPrune, func(rel string) {
			fmt.Printf("removed from phone: %s\n", rel)
		})
		if err != nil {
			return err
		}
		if len(res.Failed) > 0 {
			for rel, ferr := range res.Failed {
				fmt.Fprintf(os.Stderr, "logSync: failed to remove %s: %v\n", rel, ferr)
			}
			return fmt.Errorf("sync: %d of %d files failed to remove from phone", len(res.Failed), len(toPrune))
		}
	}

	return nil
}

// gitRel translates a path relative to localDir into the path git wants:
// one relative to the tracking repo's own root. These are only the same
// string when local_dir *is* the tracking repo. When local_dir is a
// subdirectory of it — the usual arrangement, since the merge scope is
// meant to be narrower than the repo — passing the local_dir-relative path
// straight through asked git for "journal.md" when the repo held
// "notes/journal.md" —
// so every lookup missed, the git ancestor never resolved, and
// splitOneSided judged every path untracked, which quietly disabled the
// apparent-deletion prompt entirely.
//
// ok is false when there is no tracking repo, or when localDir lies
// outside it: git has nothing to say about those paths.
func gitRel(trackingRepo, localDir, rel string) (string, bool) {
	if trackingRepo == "" {
		return "", false
	}
	abs := filepath.Join(localDir, filepath.FromSlash(rel))
	r, err := filepath.Rel(trackingRepo, abs)
	if err != nil {
		return "", false
	}
	r = filepath.ToSlash(r)
	if r == ".." || strings.HasPrefix(r, "../") {
		return "", false
	}
	return r, true
}

// resolveAncestor finds the best available 3-way-merge base for rel: a real
// git ancestor when trackingRepo tracks it, otherwise the last state this
// tool itself confirmed converged (see package snapshot — this is what
// makes merges over a merge scope that git deliberately ignores still
// get real block-preserving 3-way behavior instead of degrading to an
// empty-base union). Neither found falls back to the original empty-base
// union, unchanged.
func resolveAncestor(trackingRepo, localDir, rel string) (ancestor []byte, hasAncestor bool, err error) {
	if grel, ok := gitRel(trackingRepo, localDir, rel); ok {
		ancestor, hasAncestor, err = merge.Ancestor(trackingRepo, grel)
		if err != nil {
			return nil, false, err
		}
		if hasAncestor {
			return ancestor, true, nil
		}
	}
	return snapshot.Read(filepath.Join(localDir, filepath.FromSlash(rel)))
}

// recordConvergence remembers absPath's current content as the baseline for
// its next merge. absPath is the file's real location on this machine,
// which is also its key in the snapshot store — see snapshot.Path for why
// the key is the absolute path rather than a scope-relative one.
func recordConvergence(absPath string) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	return snapshot.Write(absPath, data)
}

func isIn(set map[string]struct{}, rel string) bool {
	_, ok := set[rel]
	return ok
}

type side int

const (
	sideLocal side = iota
	sidePhone
)

// splitOneSided sorts a one-sided (present-here, absent-there) file list
// into "just copy it across" vs "a human has to say". Per the user's
// decision: untracked paths are assumed new (copy, no prompt); paths
// tracked in the git repo are ambiguous — a file that's absent could be
// new-but-not-yet-synced or deleted-on-the-other-side.
//
// This does no prompting and no I/O beyond the git lookup, so preview and
// commit runs classify identically and the caller can print the whole plan
// before asking anything.
func splitOneSided(s *session, rels []string) (toCopy, needsDecision []string, err error) {
	if s.cfg.TrackingRepo == "" {
		return rels, nil, nil
	}
	for _, rel := range rels {
		grel, ok := gitRel(s.cfg.TrackingRepo, s.localDir, rel)
		if !ok {
			toCopy = append(toCopy, rel)
			continue
		}
		_, tracked, err := merge.Ancestor(s.cfg.TrackingRepo, grel)
		if err != nil {
			return nil, nil, fmt.Errorf("checking git history for %s: %w", rel, err)
		}
		if tracked {
			needsDecision = append(needsDecision, rel)
		} else {
			toCopy = append(toCopy, rel)
		}
	}
	return toCopy, needsDecision, nil
}

// decideOneSided asks about each ambiguous path and returns the ones to
// copy across anyway. v1 never deletes on either side, so "yes, it was
// deleted" only means "leave it alone"; nothing here can destroy content.
func decideOneSided(s *session, rels []string, presentSide side) (toCopy []string, err error) {
	if len(rels) == 0 {
		return nil, nil
	}
	if !stdinIsTerminal() {
		// A panel button, a cron entry or a piped run has nobody to answer.
		// Copying is the choice that cannot lose a note, so take it and say
		// so rather than blocking forever on a read that will never return.
		for _, rel := range rels {
			fmt.Printf("no terminal to ask on, copying across (rerun interactively to decide): %s\n", rel)
		}
		return rels, nil
	}
	reader := bufio.NewReader(os.Stdin)
	for i, rel := range rels {
		switch promptApparentDeletion(reader, s, rel, presentSide) {
		case answerCopy:
			toCopy = append(toCopy, rel)
		case answerSkip:
			// Nothing to do: v1 leaves the file where it is.
		case answerCopyAll:
			return append(toCopy, rels[i:]...), nil
		case answerSkipAll:
			return toCopy, nil
		}
	}
	return toCopy, nil
}

// answer is what a human said about one apparent deletion. The two "all"
// answers exist because a vault that has had a folder reorganised produces
// one question per file, and there was previously no way out of them but
// Ctrl-C — which aborts the whole round, including the transfers already
// decided.
type answer int

const (
	answerCopy answer = iota
	answerSkip
	answerCopyAll
	answerSkipAll
)

// isTerminal reports whether f is a real terminal, i.e. whether there is a
// human on the other end of it to answer a question.
//
// The obvious check — os.ModeCharDevice on a Stat — is wrong here: /dev/null
// is a character device too, so `sync --commit < /dev/null` passed it and
// went on to prompt anyway. Asking for the terminal attributes is the test
// that actually distinguishes them; a non-tty simply has none.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

func stdinIsTerminal() bool { return isTerminal(os.Stdin) }

// promptApparentDeletion shows enough of the note to decide with — the
// full YAML frontmatter plus a real slice of body, not a fixed line count —
// and asks whether this is a deletion to respect (report only, v1 never
// deletes) or a file that should simply be copied across.
func promptApparentDeletion(reader *bufio.Reader, s *session, rel string, presentSide side) answer {
	var presentPath, missingDesc string
	switch presentSide {
	case sideLocal:
		presentPath = filepath.Join(s.localDir, filepath.FromSlash(rel))
		missingDesc = "phone"
	case sidePhone:
		presentPath = filepath.Join(s.stageDir, filepath.FromSlash(rel))
		missingDesc = "local"
	}

	fmt.Printf("\n%s is tracked in git but missing on %s — possible deletion:\n", rel, missingDesc)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Print(previewContent(presentPath))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Treat as deleted on %s (skip, don't copy)?\n"+
		"  [y] yes, skip it   [N] no, copy it across (default)\n"+
		"  [a] copy all the rest   [s] skip all the rest\n> ", missingDesc)

	line, _ := reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return answerSkip
	case "a", "all":
		return answerCopyAll
	case "s", "skipall":
		return answerSkipAll
	default:
		return answerCopy
	}
}

// previewContent renders enough of a note to decide with: the full YAML
// frontmatter block, then body up to the first blank-line paragraph break
// or a size cap, whichever comes first.
func previewContent(path string) string {
	const bodyCap = 800
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(could not read: %v)\n", err)
	}
	text := string(data)

	var frontmatter, body string
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---\n"); end != -1 {
			cut := end + 4 + len("\n---\n")
			frontmatter = text[:cut]
			body = text[cut:]
		}
	}
	if frontmatter == "" {
		body = text
	}

	if idx := strings.Index(body, "\n\n"); idx != -1 && idx < bodyCap {
		body = body[:idx] + "\n...\n"
	} else if len(body) > bodyCap {
		body = body[:bodyCap] + "\n...\n"
	} else if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return frontmatter + body
}

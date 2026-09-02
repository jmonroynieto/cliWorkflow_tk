package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/adbx"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/atomicfile"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
	"github.com/urfave/cli/v3"
)

// pushCommand: one-way overwrite local -> phone, carried over unchanged
// from the bash tool this replaces: local files win at every shared path.
// This is a projection, not a merge, and local is never modified.
var pushCommand = &cli.Command{
	Name:  "push",
	Usage: "one-way overwrite: local files win at every shared path",
	Flags: []cli.Flag{previewFlag, commitFlag, withSymlinksFlag, withObsidianFlag, pruneFlag},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return runOneWay(cmd, oneWayDirectionLocalToPhone)
	},
}

// pullCommand: the symmetric one-way overwrite, phone -> local. The bash
// tool had no such mode; it used the name `pull` for its bidirectional one,
// which is backwards — a pull that also pushes. Here `sync` is the
// bidirectional command and `pull` does exactly what its name says.
var pullCommand = &cli.Command{
	Name:  "pull",
	Usage: "one-way overwrite: phone files win at every shared path",
	Flags: []cli.Flag{previewFlag, commitFlag, withSymlinksFlag, withObsidianFlag},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return runOneWay(cmd, oneWayDirectionPhoneToLocal)
	},
}

type oneWayDirection int

const (
	oneWayDirectionLocalToPhone oneWayDirection = iota
	oneWayDirectionPhoneToLocal
)

func runOneWay(cmd *cli.Command, dir oneWayDirection) error {
	commit := cmd.Bool("commit")

	s, err := openSession(cmd.String("config"), scopeMerge)
	if err != nil {
		return err
	}
	defer s.close()

	withObsidian := cmd.Bool("with-obsidian")
	c, err := synctree.Classify(s.localDir, s.stageDir, s.cfg.ignorePatterns(), effectiveExcludes(s.cfg, withObsidian), cmd.Bool("with-symlinks"))
	if err != nil {
		return err
	}
	c = s.report(c)
	c, cfgDiffer, cfgLocalOnly, cfgPhoneOnly := splitObsidian(c)
	defer reportObsidian(s, cfgDiffer, cfgLocalOnly, cfgPhoneOnly, nil)
	s.pruneSnapshots(c, commit)
	localSymlinks := make(map[string]struct{}, len(c.LocalSymlinks))
	for _, rel := range c.LocalSymlinks {
		localSymlinks[rel] = struct{}{}
	}

	// prune only applies pushing local -> phone: pull has no --prune flag
	// registered on it, and this short-circuits before asking the command
	// for a flag it was never given.
	prune := dir == oneWayDirectionLocalToPhone && cmd.Bool("prune")

	var toSend, toCopyIn, toPrune []string
	switch dir {
	case oneWayDirectionLocalToPhone:
		toSend = append(append([]string{}, c.Differ...), c.LocalOnly...)
		if prune {
			// Push otherwise never looks at PhoneOnly at all: those paths
			// aren't "shared", so local-wins has nothing to say about them
			// and they sit on the phone forever. --prune is the one case
			// where push does consider them, treating local's absence as
			// the file having been deleted here.
			toPrune = c.PhoneOnly
		}
	case oneWayDirectionPhoneToLocal:
		// A local symlink is never a pull target, even if it "differs" from
		// the phone's copy — local is authoritative for these paths, and
		// pull must not replace the link with a plain file.
		for _, rel := range c.Differ {
			if _, guarded := localSymlinks[rel]; guarded {
				fmt.Printf("protected (local is a symlink, not pulled): %s\n", rel)
				continue
			}
			toCopyIn = append(toCopyIn, rel)
		}
		toCopyIn = append(toCopyIn, c.PhoneOnly...)
	}

	if len(toSend) == 0 && len(toCopyIn) == 0 && len(toPrune) == 0 {
		fmt.Println("logSync: already in sync (nothing to do)")
		return nil
	}

	// Under --commit each transfer prints its own line as it lands; the
	// plan is only worth restating when nothing is going to happen.
	if !commit {
		for _, rel := range toSend {
			fmt.Printf("would send: %s\n", rel)
		}
		for _, rel := range toCopyIn {
			fmt.Printf("would copy in: %s\n", rel)
		}
		for _, rel := range toPrune {
			fmt.Printf("would remove from phone (pruned, missing locally): %s\n", rel)
		}
		fmt.Println("logSync: preview only, pass --commit to apply")
		return nil
	}

	if len(toCopyIn) > 0 {
		for _, rel := range toCopyIn {
			src := filepath.Join(s.stageDir, filepath.FromSlash(rel))
			dst := filepath.Join(s.localDir, filepath.FromSlash(rel))
			if err := atomicfile.CopyPreservingTimestamp(src, dst); err != nil {
				return fmt.Errorf("copy in %s: %w", rel, err)
			}
			fmt.Printf("copied in: %s\n", rel)
			if err := recordConvergence(dst); err != nil {
				fmt.Fprintf(os.Stderr, "logSync: warning: could not snapshot %s: %v\n", rel, err)
			}
		}
	}
	if len(toSend) > 0 {
		res, err := adbx.StageOut(s.dev, s.localDir, s.remoteDir, toSend, func(rel string) {
			fmt.Printf("sent: %s\n", rel)
		})
		if err != nil {
			return err
		}
		for _, rel := range res.Succeeded {
			localPath := filepath.Join(s.localDir, filepath.FromSlash(rel))
			if err := recordConvergence(localPath); err != nil {
				fmt.Fprintf(os.Stderr, "logSync: warning: could not snapshot %s: %v\n", rel, err)
			}
		}
		if len(res.Failed) > 0 {
			for rel, ferr := range res.Failed {
				fmt.Fprintf(os.Stderr, "logSync: failed to send %s: %v\n", rel, ferr)
			}
			return fmt.Errorf("push: %d of %d files failed, %d sent", len(res.Failed), len(toSend), len(res.Succeeded))
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
			return fmt.Errorf("push: %d of %d files failed to remove from phone", len(res.Failed), len(toPrune))
		}
	}
	return nil
}

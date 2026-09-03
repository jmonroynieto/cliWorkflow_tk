package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/adbx"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
	"github.com/urfave/cli/v3"
)

// notesyncCommand is the bash tool's other, unrelated mode: it moved the
// whole vault with `adb push --sync` and shared no merge logic at all with
// the other two. One-way, local always wins, no merge, whole vault (or
// whatever vault_local_dir/vault_remote_dir name) rather than the
// narrow merge scope sync/push/pull operate on. This is the
// command for bulk-seeding or re-stamping attachments and root-level notes
// — the thing tonight's initial device seed actually needed, done properly
// scoped instead of by accident.
var notesyncCommand = &cli.Command{
	Name:  "notesync",
	Usage: "one-way whole-vault push (local wins, no merge) — see vault_local_dir/vault_remote_dir",
	Flags: []cli.Flag{previewFlag, commitFlag, withSymlinksFlag},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return runNotesync(cmd)
	},
}

func runNotesync(cmd *cli.Command) error {
	commit := cmd.Bool("commit")

	s, err := openSession(cmd.String("config"), scopeVault)
	if err != nil {
		return err
	}
	defer s.close()

	c, err := synctree.Classify(s.localDir, s.stageDir, s.cfg.ignorePatterns(), effectiveExcludes(s.cfg, false), cmd.Bool("with-symlinks"))
	if err != nil {
		return err
	}
	c = s.report(c)
	s.pruneSnapshots(c, commit)

	toSend := append(append([]string{}, c.Differ...), c.LocalOnly...)
	if len(toSend) == 0 {
		fmt.Println("logSync: already in sync (nothing to do)")
		return nil
	}

	// Under --commit each file prints its own "sent:" line; restating the
	// plan first only shows every path twice.
	if !commit {
		for _, rel := range toSend {
			fmt.Printf("would send: %s\n", rel)
		}
		fmt.Println("logSync: preview only, pass --commit to apply")
		return nil
	}

	res, err := adbx.StageOut(s.dev, s.localDir, s.remoteDir, toSend, func(rel string) {
		fmt.Printf("sent: %s\n", rel)
	})
	if err != nil {
		return err
	}
	// Seeding a device converges every path it sends, and that convergence
	// is exactly what the next round's 3-way merge needs as a base. Without
	// this, the documented workflow — seed with notesync, sync from then on
	// — guaranteed that every note met its first two-sided merge with no
	// ancestor from either source, and the empty-base union duplicated
	// whatever content the two sides already shared.
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
		return fmt.Errorf("notesync: %d of %d files failed, %d sent", len(res.Failed), len(toSend), len(res.Succeeded))
	}
	return nil
}

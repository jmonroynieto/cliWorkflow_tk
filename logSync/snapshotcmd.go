package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/snapshot"
	"github.com/urfave/cli/v3"
)

// snapshotCommand exposes the merge-baseline store: what is in it, and how
// to get rid of what is no longer earning its keep.
//
// A sync round prunes the baselines for files it just found gone from both
// sides, but only within the directory it was syncing, and never the ones
// whose containing directory has itself disappeared — a run that cannot see
// its own vault must not conclude the vault's history is garbage. That
// leaves two cases for a human to settle: a vault that really did move or
// get deleted, and baselines written by an older build under keys that were
// never absolute paths. Both surface here.
var snapshotCommand = &cli.Command{
	Name:  "snapshot",
	Usage: "inspect or clean up the stored merge baselines",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return runSnapshotList()
	},
	Commands: []*cli.Command{
		{
			Name:  "prune",
			Usage: "remove baselines whose file is gone",
			Flags: []cli.Flag{previewFlag, commitFlag, orphansFlag},
			Action: func(ctx context.Context, cmd *cli.Command) error {
				return runSnapshotPrune(cmd.Bool("commit"), cmd.Bool("orphans"))
			},
		},
	},
}

var orphansFlag = &cli.BoolFlag{
	Name:  "orphans",
	Usage: "also remove baselines whose whole directory is gone — check the paths first, an unmounted vault looks exactly like a deleted one",
}

func runSnapshotList() error {
	dir, exists, err := snapshot.Locate()
	if err != nil {
		return err
	}
	fmt.Printf("store: %s\n", dir)
	if !exists {
		fmt.Println("store does not exist yet (nothing has converged)")
		return nil
	}

	entries, err := snapshot.List("")
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("store is empty")
		return nil
	}

	var total int64
	byState := map[snapshot.State]int{}
	bySize := map[snapshot.State]int64{}
	for _, e := range entries {
		total += e.Size
		byState[e.State]++
		bySize[e.State] += e.Size
	}
	fmt.Printf("baselines: %d, holding %s of note content\n", len(entries), humanBytes(total))
	for _, st := range []snapshot.State{snapshot.Live, snapshot.Stale, snapshot.Orphaned} {
		if byState[st] > 0 {
			fmt.Printf("  %-9s %5d  %s\n", st, byState[st], humanBytes(bySize[st]))
		}
	}

	// The paths worth eyeballing are the ones a prune would act on; live
	// baselines are just the vault again and listing them says nothing.
	printGrouped("stale (file gone, its directory still there)", entries, snapshot.Stale)
	printGrouped("orphaned (directory gone — moved vault, or one not mounted)", entries, snapshot.Orphaned)
	if byState[snapshot.Stale] > 0 || byState[snapshot.Orphaned] > 0 {
		fmt.Println("\nremove them with: logSync snapshot prune --commit" +
			"\n(orphaned ones need --orphans as well)")
	}
	return nil
}

// printGrouped lists the directories a state's entries fall under rather
// than every path, so a moved vault reads as one line instead of hundreds.
func printGrouped(heading string, entries []snapshot.Entry, want snapshot.State) {
	counts := map[string]int{}
	for _, e := range entries {
		if e.State == want {
			counts[parentDir(e.LocalPath)]++
		}
	}
	if len(counts) == 0 {
		return
	}
	dirs := make([]string, 0, len(counts))
	for d := range counts {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	fmt.Printf("\n%s:\n", heading)
	for _, d := range dirs {
		fmt.Printf("  %4d  %s\n", counts[d], d)
	}
}

func runSnapshotPrune(commit, orphans bool) error {
	_, exists, err := snapshot.Locate()
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("logSync: no snapshot store, nothing to prune")
		return nil
	}
	entries, err := snapshot.List("")
	if err != nil {
		return err
	}

	var doomed []snapshot.Entry
	var heldBack int
	for _, e := range entries {
		switch e.State {
		case snapshot.Stale:
			doomed = append(doomed, e)
		case snapshot.Orphaned:
			if orphans {
				doomed = append(doomed, e)
			} else {
				heldBack++
			}
		}
	}

	if len(doomed) == 0 {
		if heldBack > 0 {
			fmt.Printf("logSync: nothing stale; %d orphaned baseline(s) left alone (pass --orphans to include them)\n", heldBack)
			return nil
		}
		fmt.Println("logSync: nothing to prune")
		return nil
	}

	if !commit {
		for _, e := range doomed {
			fmt.Printf("would drop (%s): %s\n", e.State, e.LocalPath)
		}
		if heldBack > 0 {
			fmt.Printf("%d orphaned baseline(s) left alone (pass --orphans to include them)\n", heldBack)
		}
		fmt.Println("logSync: preview only, pass --commit to apply")
		return nil
	}

	var freed int64
	for _, e := range doomed {
		if err := snapshot.Remove(e); err != nil {
			return err
		}
		freed += e.Size
		fmt.Printf("dropped (%s): %s\n", e.State, e.LocalPath)
	}
	fmt.Printf("logSync: dropped %d baseline(s), freeing %s\n", len(doomed), humanBytes(freed))
	if heldBack > 0 {
		fmt.Printf("%d orphaned baseline(s) left alone (pass --orphans to include them)\n", heldBack)
	}
	return nil
}

func parentDir(p string) string {
	for i := len(p) - 1; i > 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "/"
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

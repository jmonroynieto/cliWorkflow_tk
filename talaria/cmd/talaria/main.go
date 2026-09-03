// Command talaria manages short-lived disposable workspaces.
//
// Prefers talariad when the Unix socket is reachable; otherwise falls back to
// the original direct JSON registry workflow. Path on stdout, status on stderr.
//
//	cd "$(talaria new "Investigate mmap")"
//	eval "$(talaria shell-init bash)"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/backend"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/doctor"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/shell"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

// Set at link time by the toolkit Makefile:
//
//	-X main.Version=1 -X main.Revision=7 -X main.CommitId=$(GIT_TAG)
var (
	Version  = "0"
	Revision = "1"
	CommitId = "unknown"
)

func main() {
	cmd := newApp()
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "talaria: %v\n", err)
		os.Exit(1)
	}
}

func newApp() *cli.Command {
	return &cli.Command{
		Name:    "talaria",
		Usage:   "Disposable workspace manager",
		Version: fmt.Sprintf("%s.%s (%s)", Version, Revision, CommitId),
		Description: `Track short-lived scratch directories with descriptions, pins, tags, and notes.

Prefers the talariad daemon when available; falls back to a local JSON
registry (same as Stage 1) when the daemon is not running.

  cd "$(talaria new "Investigate mmap")"
  eval "$(talaria shell-init bash)"`,
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "data-dir",
				Usage:   "State directory for the JSON registry",
				Sources: cli.EnvVars("TALARIA_DATA_DIR"),
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "workspace-root",
				Usage:   "Root directory for ephemeral workspace folders",
				Sources: cli.EnvVars("TALARIA_WORKSPACE_ROOT"),
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "durable-root",
				Usage:   "Root for reboot-safe workspaces (perdure / new --durable)",
				Sources: cli.EnvVars("TALARIA_DURABLE_ROOT"),
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "socket",
				Usage:   "talariad Unix socket path",
				Sources: cli.EnvVars("TALARIA_SOCKET"),
				Hidden:  true,
			},
			&cli.BoolFlag{
				Name:    "local",
				Usage:   "Force local registry (skip daemon)",
				Sources: cli.EnvVars("TALARIA_FORCE_LOCAL"),
			},
			&cli.BoolFlag{
				Name:    "require-daemon",
				Usage:   "Fail if talariad is not reachable (no local fallback)",
				Sources: cli.EnvVars("TALARIA_FORCE_DAEMON"),
			},
		},
		Before: applyGlobalPaths,
		Commands: []*cli.Command{
			cmdNew(),
			cmdList(),
			cmdShow(),
			cmdEnter(),
			cmdRemove(),
			cmdPin(),
			cmdUnpin(),
			cmdPerdure(),
			cmdGC(),
			cmdMaintain(),
			cmdTag(),
			cmdUntag(),
			cmdFind(),
			cmdNote(),
			cmdNotes(),
			cmdStatus(),
			cmdDoctor(),
			cmdShellInit(),
		},
	}
}

func applyGlobalPaths(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	if d := cmd.String("data-dir"); d != "" {
		_ = os.Setenv("TALARIA_DATA_DIR", d)
	}
	if r := cmd.String("workspace-root"); r != "" {
		_ = os.Setenv("TALARIA_WORKSPACE_ROOT", r)
	}
	if r := cmd.String("durable-root"); r != "" {
		_ = os.Setenv("TALARIA_DURABLE_ROOT", r)
	}
	if s := cmd.String("socket"); s != "" {
		_ = os.Setenv("TALARIA_SOCKET", s)
	}
	return ctx, nil
}

// withBackend opens daemon-or-local for the duration of an action.
func withBackend(fn func(ctx context.Context, cmd *cli.Command, b backend.Backend) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		opts := backend.OpenOptions{
			ForceLocal:  cmd.Bool("local"),
			ForceDaemon: cmd.Bool("require-daemon"),
		}
		b, err := backend.Open(ctx, opts)
		if err != nil {
			return err
		}
		defer b.Close()
		return fn(ctx, cmd, b)
	}
}

// --- commands ---------------------------------------------------------------

func cmdNew() *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     "Create a workspace; print path on stdout",
		ArgsUsage: `[description]`,
		Arguments: []cli.Argument{
			&cli.StringArgs{Name: "description", Min: 0, Max: -1},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "durable",
				Aliases: []string{"D"},
				Usage:   "Create under durable root (survives reboot) and pin",
			},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			desc := strings.Join(cmd.StringArgs("description"), " ")
			ws, err := b.Create(ctx, store.CreateOptions{
				Description: desc,
				Durable:     cmd.Bool("durable"),
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Created workspace\n\n")
			fmt.Fprintf(os.Stderr, "  ID:      %s\n", ws.ID)
			fmt.Fprintf(os.Stderr, "  Path:    %s\n", ws.Path)
			fmt.Fprintf(os.Stderr, "  Created: %s\n", ws.CreatedAt.Format("2006-01-02"))
			if desc != "" {
				fmt.Fprintf(os.Stderr, "  Desc:    %s\n", desc)
			}
			if cmd.Bool("durable") || paths.IsDurablePath(ws.Path) {
				fmt.Fprintf(os.Stderr, "  Durable: yes (survives reboot)\n")
			}
			fmt.Fprintf(os.Stderr, "  Backend: %s\n", b.Mode())
			fmt.Fprintln(os.Stderr)
			fmt.Println(ws.Path)
			return nil
		}),
	}
}

func cmdList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "List workspaces",
		Action: withBackend(func(ctx context.Context, _ *cli.Command, b backend.Backend) error {
			list, err := b.List(ctx)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(os.Stderr, "No workspaces.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPIN\tDUR\tLAST USED\tDESCRIPTION\tPATH")
			for _, ws := range list {
				pin := ""
				if ws.Pinned {
					pin = "*"
				}
				dur := ""
				if paths.IsDurablePath(ws.Path) {
					dur = "D"
				}
				desc := ws.Description
				if desc == "" {
					desc = "-"
				}
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					ws.ID, pin, dur, ws.LastUsed.Format("2006-01-02 15:04"), desc, ws.Path)
			}
			return tw.Flush()
		}),
	}
}

func cmdShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Aliases:   []string{"get"},
		Usage:     "Show one workspace (path, tags, notes)",
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id := cmd.StringArg("id")
			if id == "" {
				return cli.Exit("missing required argument: id", 2)
			}
			ws, err := b.Get(ctx, id)
			if err != nil {
				return err
			}
			fmt.Printf("ID:          %s\n", ws.ID)
			fmt.Printf("Path:        %s\n", ws.Path)
			fmt.Printf("Description: %s\n", emptyDash(ws.Description))
			fmt.Printf("Created:     %s\n", ws.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Last used:   %s\n", ws.LastUsed.Format(time.RFC3339))
			fmt.Printf("Pinned:      %v\n", ws.Pinned)
			fmt.Printf("Durable:     %v\n", paths.IsDurablePath(ws.Path))
			if ws.Pinned && paths.IsVolatilePath(ws.Path) && !paths.IsDurablePath(ws.Path) {
				fmt.Printf("Warning:     pinned but under volatile path (reboot will lose data); run: talaria perdure %s\n", ws.ID)
			} else if paths.IsDurablePath(ws.Path) && paths.IsVolatilePath(ws.Path) {
				fmt.Printf("Warning:     durable root is itself under volatile storage; reboot may still wipe files\n")
			}
			fmt.Printf("Backend:     %s\n", b.Mode())
			if len(ws.Tags) > 0 {
				fmt.Printf("Tags:        %s\n", strings.Join(ws.Tags, ", "))
			}
			if len(ws.Notes) > 0 {
				fmt.Println("Notes:")
				for _, n := range ws.Notes {
					fmt.Printf("  [%s] %s\n", n.CreatedAt.Format("2006-01-02"), n.Body)
				}
			}
			return nil
		}),
	}
}

func cmdEnter() *cli.Command {
	return &cli.Command{
		Name:      "enter",
		Usage:     "Print path and update last_used (for shell cd)",
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id := cmd.StringArg("id")
			if id == "" {
				return cli.Exit("missing required argument: id", 2)
			}
			ws, err := b.Enter(ctx, id)
			if err != nil {
				return err
			}
			fmt.Println(ws.Path)
			return nil
		}),
	}
}

func cmdRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Aliases:   []string{"remove", "delete"},
		Usage:     "Delete workspace metadata and directory",
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id := cmd.StringArg("id")
			if id == "" {
				return cli.Exit("missing required argument: id", 2)
			}
			if err := b.Remove(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Removed workspace %s\n", id)
			return nil
		}),
	}
}

func cmdPin() *cli.Command {
	return &cli.Command{
		Name:      "pin",
		Usage:     "Pin workspace so it survives gc",
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			return setPinned(ctx, cmd, b, true)
		}),
	}
}

func cmdUnpin() *cli.Command {
	return &cli.Command{
		Name:      "unpin",
		Usage:     "Unpin workspace",
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			return setPinned(ctx, cmd, b, false)
		}),
	}
}

func cmdPerdure() *cli.Command {
	return &cli.Command{
		Name:  "perdure",
		Usage: "Move workspace to durable storage (survives reboot); pin by default",
		Description: `Move a workspace directory from the ephemeral root (default /tmp/talaria)
onto durable disk and update the registry path.

  pin     = survives talaria gc / maintain age GC
  perdure = survives machine reboot (files leave /tmp)

By default also pins the workspace. Use --no-pin to only relocate.

  talaria perdure y4f8
  talaria new --durable "long investigation"   # create already durable
  cd "$(talaria perdure y4f8)"`,
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Usage: "Durable root directory (default: TALARIA_DURABLE_ROOT or data-dir/workspaces)",
			},
			&cli.BoolFlag{
				Name:  "no-pin",
				Usage: "Do not pin after moving (still relocates to durable storage)",
			},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id := cmd.StringArg("id")
			if id == "" {
				return cli.Exit("missing required argument: id", 2)
			}
			ws, err := b.Perdure(ctx, id, store.PerdureOptions{
				Root:  cmd.String("root"),
				NoPin: cmd.Bool("no-pin"),
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Perdured workspace %s\n\n", ws.ID)
			fmt.Fprintf(os.Stderr, "  Path:    %s\n", ws.Path)
			fmt.Fprintf(os.Stderr, "  Pinned:  %v\n", ws.Pinned)
			fmt.Fprintf(os.Stderr, "  Durable: yes (survives reboot)\n")
			fmt.Fprintf(os.Stderr, "  Backend: %s\n", b.Mode())
			fmt.Fprintln(os.Stderr)
			fmt.Println(ws.Path)
			return nil
		}),
	}
}

func setPinned(ctx context.Context, cmd *cli.Command, b backend.Backend, pinned bool) error {
	id := cmd.StringArg("id")
	if id == "" {
		return cli.Exit("missing required argument: id", 2)
	}
	if err := b.SetPinned(ctx, id, pinned); err != nil {
		return err
	}
	if pinned {
		fmt.Fprintf(os.Stderr, "Pinned %s\n", id)
	} else {
		fmt.Fprintf(os.Stderr, "Unpinned %s\n", id)
	}
	return nil
}

func cmdGC() *cli.Command {
	return &cli.Command{
		Name:  "gc",
		Usage: "Remove unpinned workspaces",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "older-than",
				Aliases: []string{"o"},
				Usage:   "Only remove workspaces last used before this age (e.g. 7d, 24h, 30m). Default: all unpinned",
			},
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Remove all unpinned workspaces (default behaviour)",
			},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			var maxAge time.Duration
			if s := cmd.String("older-than"); s != "" {
				d, err := parseDuration(s)
				if err != nil {
					return err
				}
				maxAge = d
			}
			_ = cmd.Bool("all")
			removed, err := b.GC(ctx, maxAge)
			if err != nil {
				return err
			}
			if len(removed) == 0 {
				fmt.Fprintln(os.Stderr, "Nothing to collect.")
				return nil
			}
			fmt.Fprintf(os.Stderr, "Removed %d workspace(s): %s\n",
				len(removed), strings.Join(removed, ", "))
			return nil
		}),
	}
}

func cmdMaintain() *cli.Command {
	return &cli.Command{
		Name:  "maintain",
		Usage: "Reconcile missing dirs + optional age GC (oneshot / timer job)",
		Description: `Wake-on-call maintenance pass for systemd timers.

1. Status check: drop registry entries whose directories were deleted manually.
2. Age GC: remove unpinned workspaces older than --gc-older-than (if set).
3. Report orphan directories under the workspace root (not deleted).

Intended for Type=oneshot units, e.g. hourly. Does not require a long-running
talariad — uses the daemon when up, otherwise the local registry.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "gc-older-than",
				Aliases: []string{"o"},
				Usage:   "Also remove unpinned workspaces older than this (e.g. 7d). Empty skips age GC",
				Value:   "7d",
			},
			&cli.BoolFlag{
				Name:  "skip-gc",
				Usage: "Only reconcile missing directories; do not age-GC",
			},
			&cli.BoolFlag{
				Name:  "skip-orphans",
				Usage: "Do not scan for untracked directories under the workspace root",
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Only print a one-line summary (good for journald)",
			},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			opts := store.MaintainOptions{
				SkipGC:         cmd.Bool("skip-gc"),
				SkipOrphanScan: cmd.Bool("skip-orphans"),
			}
			if s := cmd.String("gc-older-than"); s != "" && !opts.SkipGC {
				d, err := parseDuration(s)
				if err != nil {
					return err
				}
				opts.GCOlderThan = d
			}
			res, err := b.Maintain(ctx, opts)
			if err != nil {
				return err
			}
			if cmd.Bool("quiet") {
				fmt.Printf("maintain backend=%s missing=%d gc=%d orphans=%d kept=%d\n",
					b.Mode(), len(res.Missing), len(res.GC), len(res.Orphans), res.Kept)
				return nil
			}
			fmt.Fprintf(os.Stderr, "Backend:   %s\n", b.Mode())
			if len(res.Missing) == 0 {
				fmt.Fprintln(os.Stderr, "Reconcile: no missing workspace dirs")
			} else {
				fmt.Fprintf(os.Stderr, "Reconcile: dropped %d registry entr(y/ies) (dirs gone): %s\n",
					len(res.Missing), strings.Join(res.Missing, ", "))
			}
			if opts.SkipGC || opts.GCOlderThan <= 0 {
				fmt.Fprintln(os.Stderr, "GC:        skipped")
			} else if len(res.GC) == 0 {
				fmt.Fprintf(os.Stderr, "GC:        nothing older than %s\n", cmd.String("gc-older-than"))
			} else {
				fmt.Fprintf(os.Stderr, "GC:        removed %d: %s\n", len(res.GC), strings.Join(res.GC, ", "))
			}
			if opts.SkipOrphanScan {
				fmt.Fprintln(os.Stderr, "Orphans:   skipped")
			} else if len(res.Orphans) == 0 {
				fmt.Fprintln(os.Stderr, "Orphans:   none")
			} else {
				fmt.Fprintf(os.Stderr, "Orphans:   %d untracked dir(s) under workspace root:\n", len(res.Orphans))
				for _, p := range res.Orphans {
					fmt.Fprintf(os.Stderr, "  %s\n", p)
				}
			}
			fmt.Fprintf(os.Stderr, "Kept:      %d workspace(s)\n", res.Kept)
			// Quiet machine line on stdout for scripts.
			fmt.Printf("maintain backend=%s missing=%d gc=%d orphans=%d kept=%d\n",
				b.Mode(), len(res.Missing), len(res.GC), len(res.Orphans), res.Kept)
			return nil
		}),
	}
}

func cmdTag() *cli.Command {
	return &cli.Command{
		Name:      "tag",
		Usage:     "Attach a tag to a workspace",
		ArgsUsage: `<id> <tag>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
			&cli.StringArg{Name: "tag"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id, tag := cmd.StringArg("id"), cmd.StringArg("tag")
			if id == "" || tag == "" {
				return cli.Exit("usage: talaria tag <id> <tag>", 2)
			}
			if err := b.AddTag(ctx, id, tag); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Tagged %s with %q\n", id, tag)
			return nil
		}),
	}
}

func cmdUntag() *cli.Command {
	return &cli.Command{
		Name:      "untag",
		Usage:     "Detach a tag from a workspace",
		ArgsUsage: `<id> <tag>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
			&cli.StringArg{Name: "tag"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id, tag := cmd.StringArg("id"), cmd.StringArg("tag")
			if id == "" || tag == "" {
				return cli.Exit("usage: talaria untag <id> <tag>", 2)
			}
			if err := b.RemoveTag(ctx, id, tag); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Removed tag %q from %s\n", tag, id)
			return nil
		}),
	}
}

func cmdFind() *cli.Command {
	return &cli.Command{
		Name:      "find",
		Usage:     "List workspaces with a given tag",
		ArgsUsage: `<tag>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "tag"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			tag := cmd.StringArg("tag")
			if tag == "" {
				return cli.Exit("missing required argument: tag", 2)
			}
			list, err := b.FindByTag(ctx, tag)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(os.Stderr, "No matching workspaces.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tDESCRIPTION\tPATH")
			for _, ws := range list {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", ws.ID, emptyDash(ws.Description), ws.Path)
			}
			return tw.Flush()
		}),
	}
}

func cmdNote() *cli.Command {
	return &cli.Command{
		Name:      "note",
		Usage:     "Append a note to a workspace",
		ArgsUsage: `<id> <text...>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
			&cli.StringArgs{Name: "text", Min: 1, Max: -1},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id := cmd.StringArg("id")
			parts := cmd.StringArgs("text")
			if id == "" || len(parts) == 0 {
				return cli.Exit("usage: talaria note <id> <text>", 2)
			}
			n, err := b.AddNote(ctx, id, strings.Join(parts, " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Note #%d added to %s\n", n.ID, id)
			return nil
		}),
	}
}

func cmdNotes() *cli.Command {
	return &cli.Command{
		Name:      "notes",
		Usage:     "List notes on a workspace",
		ArgsUsage: `<id>`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id"},
		},
		Action: withBackend(func(ctx context.Context, cmd *cli.Command, b backend.Backend) error {
			id := cmd.StringArg("id")
			if id == "" {
				return cli.Exit("missing required argument: id", 2)
			}
			notes, err := b.NotesFor(ctx, id)
			if err != nil {
				return err
			}
			if len(notes) == 0 {
				fmt.Fprintln(os.Stderr, "No notes.")
				return nil
			}
			for _, n := range notes {
				fmt.Printf("[%d] %s  %s\n", n.ID, n.CreatedAt.Format("2006-01-02 15:04"), n.Body)
			}
			return nil
		}),
	}
}

func cmdStatus() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show which backend would be used (daemon or local)",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			sock, _ := paths.SocketPath()
			reg, _ := paths.RegistryPath()
			opts := backend.OpenOptions{
				ForceLocal:  cmd.Bool("local"),
				ForceDaemon: cmd.Bool("require-daemon"),
				Quiet:       true,
			}
			b, err := backend.Open(ctx, opts)
			if err != nil {
				fmt.Printf("Backend:  unavailable\n")
				fmt.Printf("Error:    %v\n", err)
				fmt.Printf("Socket:   %s\n", sock)
				fmt.Printf("Registry: %s\n", reg)
				return err
			}
			defer b.Close()
			fmt.Printf("Backend:  %s\n", b.Mode())
			fmt.Printf("Socket:   %s\n", sock)
			fmt.Printf("Registry: %s\n", reg)
			if err := b.Ping(ctx); err != nil {
				fmt.Printf("Ping:     %v\n", err)
				return err
			}
			fmt.Printf("Ping:     ok\n")
			return nil
		},
	}
}

func cmdDoctor() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Diagnose socket, lock, registry, and workspace root",
		Description: `Check that talaria can run correctly:

  - socket reachable (talariad ping)
  - registry lock free or held (and by which PID when known)
  - registry JSON parseable
  - workspace root writable

When the lock is stuck after a crash:

  fuser -v ~/.local/share/talaria/workspaces.json.lock
  lsof   ~/.local/share/talaria/workspaces.json.lock
  kill $(cat $XDG_RUNTIME_DIR/talaria.pid)   # kill stale talariad
  rm -f $XDG_RUNTIME_DIR/talaria.sock $XDG_RUNTIME_DIR/talaria.pid

talariad writes a PID file next to the socket on start and removes it on exit.`,
		Action: func(ctx context.Context, _ *cli.Command) error {
			r, err := doctor.Run(ctx)
			if err != nil {
				return err
			}
			fmt.Print(doctor.Format(r))
			if !r.Healthy() {
				return cli.Exit("", 1)
			}
			return nil
		},
	}
}

func cmdShellInit() *cli.Command {
	return &cli.Command{
		Name:      "shell-init",
		Usage:     "Emit shell integration (bash|zsh|fish)",
		ArgsUsage: `[shell]`,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "shell", Value: "bash"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			sh := cmd.StringArg("shell")
			if sh == "" {
				sh = "bash"
			}
			script, err := shell.InitScript(sh)
			if err != nil {
				return err
			}
			fmt.Print(script)
			return nil
		},
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

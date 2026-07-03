package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/gromula/state"
	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

var (
	Version  string
	Revision = ".0"
	CommitId string
)

func main() {
	cmd := &cli.Command{
		Name:    "gromula",
		Usage:   "Safe, trackable PATH management as a shell intermediary",
		Version: fmt.Sprintf("%s%s (%s)", Version, Revision, CommitId),
		Commands: []*cli.Command{
			addCmd(),
			removeCmd(),
			cleanCmd(),
			showCmd(),
			historyCmd(),
			initCmd(),
			resetCmd(),
			pathCmd(),
		},
	}

	err := cmd.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func addCmd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add one or more paths to PATH (default: append)",
		ArgsUsage: "<path> [path...]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "prepend",
				Aliases: []string{"p"},
				Usage:   "Prepend instead of append (highest precedence)",
			},
			&cli.StringFlag{
				Name:  "source",
				Usage: "Identification of the caller (e.g. script path or $0)",
				Value: "cli",
			},
			&cli.StringFlag{
				Name:  "session-id",
				Usage: "Session identifier (defaults to $GROMULA_SESSION_ID)",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			paths := c.Args().Slice()
			if len(paths) == 0 {
				return fmt.Errorf("at least one path is required")
			}

			st, err := state.Load()
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}
			if len(st.Entries) == 0 {
				st.Entries = state.SeedFromEnv()
			}

			sessionID := c.String("session-id")
			if sessionID == "" {
				sessionID = os.Getenv("GROMULA_SESSION_ID")
			}

			source := c.String("source")
			prepend := c.Bool("prepend")

			newPath, err := state.ApplyAdd(st, paths, source, sessionID, prepend)
			if err != nil {
				return err
			}

			fmt.Print(newPath)
			return nil
		},
	}
}

func removeCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove one or more paths from PATH",
		ArgsUsage: "<path> [path...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Identification of the caller",
				Value: "cli",
			},
			&cli.StringFlag{
				Name:  "session-id",
				Usage: "Session identifier",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			paths := c.Args().Slice()
			if len(paths) == 0 {
				return fmt.Errorf("at least one path is required")
			}

			st, err := state.Load()
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			sessionID := c.String("session-id")
			if sessionID == "" {
				sessionID = os.Getenv("GROMULA_SESSION_ID")
			}

			source := c.String("source")

			newPath, err := state.ApplyRemove(st, paths, source, sessionID)
			if err != nil {
				return err
			}

			fmt.Print(newPath)
			return nil
		},
	}
}

func cleanCmd() *cli.Command {
	return &cli.Command{
		Name:  "clean",
		Usage: "Deduplicate the current PATH from the environment (no state change)",
		Action: func(ctx context.Context, c *cli.Command) error {
			cleaned := state.ApplyClean()
			fmt.Print(cleaned)
			return nil
		},
	}
}

// pathCmd prints ONLY the current tracked PATH string (bash-ready, no extra output).
// Use: export PATH="$(gromula path)"
func pathCmd() *cli.Command {
	return &cli.Command{
		Name:  "path",
		Usage: "Print the current tracked PATH (clean string, suitable for export)",
		Action: func(ctx context.Context, c *cli.Command) error {
			st, err := state.Load()
			if err != nil {
				return err
			}

			if len(st.Entries) == 0 {
				// No state yet → fall back to current environment PATH
				fmt.Print(os.Getenv("PATH"))
				return nil
			}

			fmt.Print(state.BuildPathString(st.Entries))
			return nil
		},
	}
}

func showCmd() *cli.Command {
	return &cli.Command{
		Name:  "show",
		Usage: "Display current tracked PATH entries with provenance",
		Action: func(ctx context.Context, c *cli.Command) error {
			st, err := state.Load()
			if err != nil {
				return err
			}

			if len(st.Entries) == 0 {
				fmt.Println("(no tracked entries - state is empty or not yet initialized)")
				return nil
			}

			fmt.Printf("gromula tracked PATH (%d entries, session: %s)\n\n", len(st.Entries), st.SessionID)
			for i, e := range st.Entries {
				fmt.Printf("%2d. %-40s  source=%s  action=%s  %s\n",
					i+1, e.Path, e.Source, e.Action, e.Timestamp.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
}

func historyCmd() *cli.Command {
	return &cli.Command{
		Name:    "history",
		Aliases: []string{"log"},
		Usage:   "Show recent operations from the audit log (JSONL)",
		Action: func(ctx context.Context, c *cli.Command) error {
			lp, err := getLogPathForDisplay()
			if err != nil {
				return err
			}
			fmt.Printf("Operations log: %s\n", lp)
			fmt.Println("Use: tail -n 50 -f", lp)
			fmt.Println("Or:  jq .", lp)
			return nil
		},
	}
}

func initCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize state directory and optionally start a new session",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "session-id",
				Usage: "Explicit session identifier to record",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			st, err := state.Load()
			if err != nil {
				return err
			}

			if len(st.Entries) == 0 {
				st.Entries = state.SeedFromEnv()
			}

			sid := c.String("session-id")
			if sid == "" {
				sid = os.Getenv("GROMULA_SESSION_ID")
			}
			st.SessionID = sid

			if err := state.Save(st); err != nil {
				return err
			}

			fmt.Printf("gromula state initialized at %s\n", mustStateDir())
			if sid != "" {
				fmt.Printf("Session ID recorded: %s\n", sid)
			}
			return nil
		},
	}
}

func resetCmd() *cli.Command {
	return &cli.Command{
		Name:  "reset",
		Usage: "Clear all tracked state (does NOT modify your current PATH)",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "confirm",
				Aliases: []string{"c"},
				Usage:   "safety bool",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			dir, err := getStateDirForReset()
			if err != nil {
				return err
			}
			fmt.Printf("Would remove: %s\n", dir)
			switch c.Bool("confirm") {
			case true:
				fmt.Print("Are you sure you want to continue? [y/N]: ")
				var response string
				fmt.Scanln(&response)

				resp := strings.ToLower(strings.TrimSpace(response))
				if resp != "y" && resp != "yes" {
					fmt.Println("Aborted. No changes made.")
					return nil
				}

				if err := os.RemoveAll(dir); err != nil {
					return fmt.Errorf("failed to remove state directory: %w", err)
				}

				fmt.Printf("✓ Removed: %s\n", dir)
				fmt.Println("gromula tracking has been reset. Your actual $PATH was not modified.")
				return nil
			case false:
				fmt.Println("Run with confirmation in a future version, or manually rm -rf the directory.")
				return nil
			}
			return nil
		},
	}
}

// Small helpers for commands that need the state dir path for display
func mustStateDir() string {
	d, _ := stateLoadStateDir()
	return d
}

func getLogPathForDisplay() (string, error) {
	return stateLogPath()
}

// The following are small adapters because the state package keeps helpers unexported for cleanliness.
// In a real refactor we would export a couple of helpers, but for v1 this works.
func stateLoadStateDir() (string, error) {
	// We re-implement minimal logic here for display commands only
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, _ := os.UserHomeDir()
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "gromula"), nil
}

func stateLogPath() (string, error) {
	d, err := stateLoadStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "operations.jsonl"), nil
}

func getStateDirForReset() (string, error) {
	return stateLoadStateDir()
}

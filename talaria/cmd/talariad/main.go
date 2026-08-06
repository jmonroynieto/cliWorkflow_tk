// Command talariad is the talaria daemon: sole long-lived writer of the
// workspace registry, serving CLI (and future TUI/GUI) clients over a Unix
// socket.
//
//	talariad
//	talariad --gc-interval 1m --gc-older-than 7d
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/daemon"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
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
	cmd := &cli.Command{
		Name:    "talariad",
		Usage:   "Talaria workspace daemon",
		Version: fmt.Sprintf("%s.%s (%s)", Version, Revision, CommitId),
		Description: `Owns the JSON registry and serves clients over a Unix socket.

The CLI prefers this daemon when it is reachable, and falls back to direct
local registry access when it is not.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "socket",
				Usage:   "Unix socket path",
				Sources: cli.EnvVars("TALARIA_SOCKET"),
			},
			&cli.StringFlag{
				Name:    "data-dir",
				Usage:   "State directory (registry lives here)",
				Sources: cli.EnvVars("TALARIA_DATA_DIR"),
			},
			&cli.StringFlag{
				Name:    "workspace-root",
				Usage:   "Root directory for ephemeral workspace folders",
				Sources: cli.EnvVars("TALARIA_WORKSPACE_ROOT"),
			},
			&cli.StringFlag{
				Name:    "durable-root",
				Usage:   "Root for reboot-safe workspaces",
				Sources: cli.EnvVars("TALARIA_DURABLE_ROOT"),
			},
			&cli.DurationFlag{
				Name:  "gc-interval",
				Usage: "Background GC interval (0 disables)",
				Value: time.Minute,
			},
			&cli.StringFlag{
				Name:  "gc-older-than",
				Usage: "Only auto-GC unpinned workspaces older than this (e.g. 7d). Empty disables age-based auto-GC",
				Value: "7d",
			},
		},
		Action: run,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "talariad: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
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

	var older time.Duration
	if s := cmd.String("gc-older-than"); s != "" {
		d, err := parseDuration(s)
		if err != nil {
			return err
		}
		older = d
	}

	// If older-than is empty we disable the ticker entirely unless user
	// explicitly wants interval-only (they can pass a huge age).
	interval := cmd.Duration("gc-interval")
	if older <= 0 {
		interval = 0
	}

	sock, _ := paths.SocketPath()
	reg, _ := paths.RegistryPath()

	cfg := daemon.Config{
		Version:      fmt.Sprintf("%s.%s (%s)", Version, Revision, CommitId),
		SocketPath:   sock,
		RegistryPath: reg,
		GCInterval:   interval,
		GCOlderThan:  older,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	return daemon.Run(ctx, cfg)
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) > 0 && s[len(s)-1] == 'd' {
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

// Package daemon runs talariad: owns the JSON registry, serves the Unix
// socket API, and optionally runs background GC.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/rpc"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

// Config for talariad.
type Config struct {
	Version string
	// SocketPath overrides paths.SocketPath when non-empty.
	SocketPath string
	// RegistryPath overrides paths.RegistryPath when non-empty.
	RegistryPath string
	// GCInterval is how often the background collector runs. 0 disables it.
	GCInterval time.Duration
	// GCOlderThan is passed to store.GC. 0 means "all unpinned" on each tick
	// (dangerous); prefer a positive age such as 7*24h. Ignored if GCInterval is 0.
	GCOlderThan time.Duration
	// Logf is optional (defaults to stderr).
	Logf func(format string, args ...any)
}

// Run starts the daemon and blocks until ctx is cancelled or a fatal error.
//
// Lifecycle (one cancellation tree):
//
//	ctx cancelled → accept loop stops → GC stops → store closed → socket removed
func Run(ctx context.Context, cfg Config) error {
	logf := cfg.Logf
	if logf == nil {
		logf = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "talariad: "+format+"\n", args...)
		}
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}

	regPath := cfg.RegistryPath
	if regPath == "" {
		var err error
		regPath, err = paths.RegistryPath()
		if err != nil {
			return err
		}
	}
	sockPath := cfg.SocketPath
	if sockPath == "" {
		var err error
		sockPath, err = paths.SocketPath()
		if err != nil {
			return err
		}
	}

	if err := paths.EnsureDir(filepath.Dir(regPath)); err != nil {
		return err
	}
	if err := paths.EnsureDir(filepath.Dir(sockPath)); err != nil {
		return err
	}

	// Sole long-lived writer: hold the registry lock for the daemon lifetime.
	st, err := store.Open(regPath)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer st.Close()

	// Remove stale socket / pid from a previous crash.
	_ = os.Remove(sockPath)
	pidPath := paths.PIDFileBeside(sockPath)
	_ = os.Remove(pidPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", sockPath, err)
	}
	// Restrict to the owning user.
	_ = os.Chmod(sockPath, 0o600)

	// PID file next to the socket for doctor / stale-daemon diagnosis.
	if err := writePIDFile(pidPath); err != nil {
		_ = ln.Close()
		_ = os.Remove(sockPath)
		return fmt.Errorf("pid file %s: %w", pidPath, err)
	}

	defer func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
		_ = os.Remove(pidPath)
	}()

	logf("listening on %s (registry %s, pid %d)", sockPath, regPath, os.Getpid())

	handler := &rpc.Handler{Store: st, Version: cfg.Version}

	g, ctx := errgroup.WithContext(ctx)

	// Accept loop.
	g.Go(func() error {
		return serve(ctx, ln, handler)
	})

	// Background GC (optional).
	if cfg.GCInterval > 0 {
		interval := cfg.GCInterval
		older := cfg.GCOlderThan
		g.Go(func() error {
			return garbageCollector(ctx, st, interval, older, logf)
		})
	}

	// Close the listener when ctx is cancelled so Accept unblocks.
	g.Go(func() error {
		<-ctx.Done()
		_ = ln.Close()
		return nil
	})

	err = g.Wait()
	if err != nil && ctx.Err() != nil {
		// Expected on shutdown.
		logf("shutdown")
		return nil
	}
	return err
}

func writePIDFile(path string) error {
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	// 0644 so doctor (same user) can read; content is only a PID.
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func serve(ctx context.Context, ln net.Listener, h *rpc.Handler) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Shutdown closes the listener; Accept then fails.
			if ctx.Err() != nil || isListenerClosed(err) {
				return nil
			}
			return err
		}
		go h.ServeConn(ctx, conn)
	}
}

func isListenerClosed(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return errors.Is(op.Err, net.ErrClosed) || errors.Is(op.Err, os.ErrClosed)
	}
	return false
}

// garbageCollector runs Maintain on a ticker: reconcile missing dirs, then age GC.
// Prefer the systemd oneshot timer for "wake on the hour" deployments; this
// in-process loop is for a long-lived talariad without systemd.
func garbageCollector(ctx context.Context, st *store.Store, interval, olderThan time.Duration, logf func(string, ...any)) error {
	t := time.NewTicker(interval)
	defer t.Stop()
	// Run once shortly after start so boot is covered without waiting a full interval.
	run := func() {
		res, err := st.Maintain(store.MaintainOptions{GCOlderThan: olderThan})
		if err != nil {
			logf("maintain error: %v", err)
			return
		}
		if len(res.Missing) > 0 {
			logf("reconcile: dropped %d missing workspace(s): %v", len(res.Missing), res.Missing)
		}
		if len(res.GC) > 0 {
			logf("gc: removed %d workspace(s): %v", len(res.GC), res.GC)
		}
		if len(res.Orphans) > 0 {
			logf("status: %d orphan dir(s) under workspace root (not in registry)", len(res.Orphans))
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			run()
		}
	}
}

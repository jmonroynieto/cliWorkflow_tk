package backend

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
)

// OpenOptions controls how the CLI resolves a backend.
type OpenOptions struct {
	// ForceLocal skips the daemon and uses the JSON registry directly.
	ForceLocal bool
	// ForceDaemon requires the daemon; no local fallback.
	ForceDaemon bool
	// DialTimeout bounds the attempt to reach talariad.
	DialTimeout time.Duration
	// Quiet suppresses the stderr notice when falling back to local.
	Quiet bool
}

// Open prefers talariad when reachable; otherwise falls back to the local
// JSON registry (the original Stage 1 workflow).
//
// Resolution order:
//  1. TALARIA_FORCE_LOCAL=1 or opts.ForceLocal → local
//  2. Dial Unix socket; on success → daemon backend
//  3. If ForceDaemon → error
//  4. Else → local (with optional notice on stderr)
func Open(ctx context.Context, opts OpenOptions) (Backend, error) {
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 300 * time.Millisecond
	}
	if envTruthy(os.Getenv("TALARIA_FORCE_LOCAL")) {
		opts.ForceLocal = true
	}
	if envTruthy(os.Getenv("TALARIA_FORCE_DAEMON")) {
		opts.ForceDaemon = true
	}

	if opts.ForceLocal {
		return OpenLocal(ctx)
	}

	socket, err := paths.SocketPath()
	if err != nil {
		if opts.ForceDaemon {
			return nil, err
		}
		return openLocalFallback(ctx, opts, err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
	defer cancel()
	remote, err := DialRemote(dialCtx, socket)
	if err == nil {
		return remote, nil
	}

	if opts.ForceDaemon {
		return nil, fmt.Errorf("talariad not reachable at %s: %w", socket, err)
	}
	return openLocalFallback(ctx, opts, err)
}

func openLocalFallback(ctx context.Context, opts OpenOptions, dialErr error) (Backend, error) {
	if !opts.Quiet && envTruthy(os.Getenv("TALARIA_VERBOSE")) {
		fmt.Fprintf(os.Stderr, "talaria: daemon unavailable (%v); using local registry\n", dialErr)
	}
	return OpenLocal(ctx)
}

func envTruthy(v string) bool {
	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}

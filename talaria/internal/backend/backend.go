// Package backend is the CLI/daemon boundary for workspace operations.
//
// The CLI always talks to a Backend:
//   - Remote: dial talariad (preferred when the daemon is up)
//   - Local:  direct JSON registry access (fallback — the original workflow)
package backend

import (
	"context"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

// Mode names for diagnostics.
const (
	ModeDaemon = "daemon"
	ModeLocal  = "local"
)

// Backend is the transport-agnostic workspace API.
// Both the local JSON store adapter and the daemon client implement it.
type Backend interface {
	Mode() string

	Create(ctx context.Context, opts store.CreateOptions) (*store.Workspace, error)
	List(ctx context.Context) ([]store.Workspace, error)
	Get(ctx context.Context, id string) (*store.Workspace, error)
	// Enter returns the workspace after updating last_used.
	Enter(ctx context.Context, id string) (*store.Workspace, error)
	Remove(ctx context.Context, id string) error
	SetPinned(ctx context.Context, id string, pinned bool) error
	// Perdure moves a workspace onto durable storage (survives reboot).
	Perdure(ctx context.Context, id string, opts store.PerdureOptions) (*store.Workspace, error)
	GC(ctx context.Context, maxAge time.Duration) ([]string, error)
	// Maintain reconciles missing dirs + optional age GC (timer / oneshot job).
	Maintain(ctx context.Context, opts store.MaintainOptions) (*store.MaintainResult, error)
	AddTag(ctx context.Context, id, tag string) error
	RemoveTag(ctx context.Context, id, tag string) error
	FindByTag(ctx context.Context, tag string) ([]store.Workspace, error)
	AddNote(ctx context.Context, id, body string) (*store.Note, error)
	NotesFor(ctx context.Context, id string) ([]store.Note, error)

	// Ping verifies the backend is reachable (no-op for local).
	Ping(ctx context.Context) error
	Close() error
}

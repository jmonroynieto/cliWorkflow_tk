package backend

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

// Local is the original single-process workflow: open JSON registry,
// hold flock for the command, close. Used when talariad is unreachable.
type Local struct {
	st *store.Store
}

// OpenLocal opens the registry with a bounded wait for the flock so a
// live daemon cannot hang the CLI forever.
func OpenLocal(ctx context.Context) (*Local, error) {
	regPath, err := paths.RegistryPath()
	if err != nil {
		return nil, err
	}
	// Bound lock wait if caller did not already.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	st, err := store.OpenContext(ctx, regPath)
	if err != nil {
		return nil, fmt.Errorf("local registry: %w (is talariad holding the lock?)", err)
	}
	return &Local{st: st}, nil
}

func (l *Local) Mode() string { return ModeLocal }

func (l *Local) Create(_ context.Context, opts store.CreateOptions) (*store.Workspace, error) {
	return l.st.CreateOpts(opts)
}

func (l *Local) Perdure(_ context.Context, id string, opts store.PerdureOptions) (*store.Workspace, error) {
	return l.st.Perdure(id, opts)
}

func (l *Local) List(_ context.Context) ([]store.Workspace, error) {
	return l.st.List()
}

func (l *Local) Get(_ context.Context, id string) (*store.Workspace, error) {
	return l.st.Get(id)
}

func (l *Local) Enter(_ context.Context, id string) (*store.Workspace, error) {
	ws, err := l.st.Get(id)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(ws.Path); err != nil {
		return nil, fmt.Errorf("workspace directory missing: %s (try talaria rm %s)", ws.Path, ws.ID)
	}
	if err := l.st.Touch(ws.ID); err != nil {
		return nil, err
	}
	return l.st.Get(id)
}

func (l *Local) Remove(_ context.Context, id string) error {
	return l.st.Remove(id)
}

func (l *Local) SetPinned(_ context.Context, id string, pinned bool) error {
	return l.st.SetPinned(id, pinned)
}

func (l *Local) GC(_ context.Context, maxAge time.Duration) ([]string, error) {
	return l.st.GC(maxAge)
}

func (l *Local) Maintain(_ context.Context, opts store.MaintainOptions) (*store.MaintainResult, error) {
	return l.st.Maintain(opts)
}

func (l *Local) AddTag(_ context.Context, id, tag string) error {
	return l.st.AddTag(id, tag)
}

func (l *Local) RemoveTag(_ context.Context, id, tag string) error {
	return l.st.RemoveTag(id, tag)
}

func (l *Local) FindByTag(_ context.Context, tag string) ([]store.Workspace, error) {
	return l.st.FindByTag(tag)
}

func (l *Local) AddNote(_ context.Context, id, body string) (*store.Note, error) {
	return l.st.AddNote(id, body)
}

func (l *Local) NotesFor(_ context.Context, id string) ([]store.Note, error) {
	return l.st.NotesFor(id)
}

func (l *Local) Ping(_ context.Context) error { return nil }

func (l *Local) Close() error {
	return l.st.Close()
}

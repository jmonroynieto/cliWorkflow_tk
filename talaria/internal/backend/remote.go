package backend

import (
	"context"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/rpc"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

// Remote talks to talariad over the Unix socket.
type Remote struct {
	client *rpc.Client
}

// DialRemote connects to the daemon socket.
func DialRemote(ctx context.Context, socketPath string) (*Remote, error) {
	c, err := rpc.Dial(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	r := &Remote{client: c}
	if err := r.Ping(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return r, nil
}

func (r *Remote) Mode() string { return ModeDaemon }

func (r *Remote) Ping(ctx context.Context) error {
	var out rpc.PingResult
	return r.client.Call(ctx, rpc.MethodPing, nil, &out)
}

func (r *Remote) Create(ctx context.Context, opts store.CreateOptions) (*store.Workspace, error) {
	var dto rpc.WorkspaceDTO
	err := r.client.Call(ctx, rpc.MethodCreate, rpc.CreateParams{
		Description: opts.Description,
		Durable:     opts.Durable,
	}, &dto)
	if err != nil {
		return nil, err
	}
	return rpc.WorkspaceFromDTO(dto), nil
}

func (r *Remote) Perdure(ctx context.Context, id string, opts store.PerdureOptions) (*store.Workspace, error) {
	var dto rpc.WorkspaceDTO
	err := r.client.Call(ctx, rpc.MethodPerdure, rpc.PerdureParams{
		ID:    id,
		Root:  opts.Root,
		NoPin: opts.NoPin,
	}, &dto)
	if err != nil {
		return nil, err
	}
	return rpc.WorkspaceFromDTO(dto), nil
}

func (r *Remote) List(ctx context.Context) ([]store.Workspace, error) {
	var out rpc.ListResult
	if err := r.client.Call(ctx, rpc.MethodList, nil, &out); err != nil {
		return nil, err
	}
	return rpc.WorkspacesFromDTO(out.Workspaces), nil
}

func (r *Remote) Get(ctx context.Context, id string) (*store.Workspace, error) {
	var dto rpc.WorkspaceDTO
	if err := r.client.Call(ctx, rpc.MethodGet, rpc.IDParams{ID: id}, &dto); err != nil {
		return nil, err
	}
	return rpc.WorkspaceFromDTO(dto), nil
}

func (r *Remote) Enter(ctx context.Context, id string) (*store.Workspace, error) {
	var dto rpc.WorkspaceDTO
	if err := r.client.Call(ctx, rpc.MethodEnter, rpc.IDParams{ID: id}, &dto); err != nil {
		return nil, err
	}
	return rpc.WorkspaceFromDTO(dto), nil
}

func (r *Remote) Remove(ctx context.Context, id string) error {
	return r.client.Call(ctx, rpc.MethodRemove, rpc.IDParams{ID: id}, nil)
}

func (r *Remote) SetPinned(ctx context.Context, id string, pinned bool) error {
	return r.client.Call(ctx, rpc.MethodSetPinned, rpc.SetPinnedParams{ID: id, Pinned: pinned}, nil)
}

func (r *Remote) GC(ctx context.Context, maxAge time.Duration) ([]string, error) {
	var out rpc.GCResult
	if err := r.client.Call(ctx, rpc.MethodGC, rpc.GCParams{MaxAgeNanos: int64(maxAge)}, &out); err != nil {
		return nil, err
	}
	return out.Removed, nil
}

func (r *Remote) Maintain(ctx context.Context, opts store.MaintainOptions) (*store.MaintainResult, error) {
	var out store.MaintainResult
	params := rpc.MaintainParams{
		GCOlderThanNanos: int64(opts.GCOlderThan),
		SkipGC:           opts.SkipGC,
		SkipOrphanScan:   opts.SkipOrphanScan,
	}
	if err := r.client.Call(ctx, rpc.MethodMaintain, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Remote) AddTag(ctx context.Context, id, tag string) error {
	return r.client.Call(ctx, rpc.MethodAddTag, rpc.TagParams{ID: id, Tag: tag}, nil)
}

func (r *Remote) RemoveTag(ctx context.Context, id, tag string) error {
	return r.client.Call(ctx, rpc.MethodRemoveTag, rpc.TagParams{ID: id, Tag: tag}, nil)
}

func (r *Remote) FindByTag(ctx context.Context, tag string) ([]store.Workspace, error) {
	var out rpc.ListResult
	if err := r.client.Call(ctx, rpc.MethodFindByTag, rpc.FindParams{Tag: tag}, &out); err != nil {
		return nil, err
	}
	return rpc.WorkspacesFromDTO(out.Workspaces), nil
}

func (r *Remote) AddNote(ctx context.Context, id, body string) (*store.Note, error) {
	var dto rpc.NoteDTO
	if err := r.client.Call(ctx, rpc.MethodAddNote, rpc.NoteParams{ID: id, Body: body}, &dto); err != nil {
		return nil, err
	}
	return rpc.NoteFromDTO(dto), nil
}

func (r *Remote) NotesFor(ctx context.Context, id string) ([]store.Note, error) {
	var out rpc.NotesResult
	if err := r.client.Call(ctx, rpc.MethodNotesFor, rpc.IDParams{ID: id}, &out); err != nil {
		return nil, err
	}
	notes := make([]store.Note, len(out.Notes))
	for i, n := range out.Notes {
		notes[i] = *rpc.NoteFromDTO(n)
	}
	return notes, nil
}

func (r *Remote) Close() error {
	return r.client.Close()
}

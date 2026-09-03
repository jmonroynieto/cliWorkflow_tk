// Package rpc is a minimal line-delimited JSON protocol over a Unix socket.
//
// It intentionally mirrors the shape of a future gRPC Talaria service so the
// daemon/CLI split is stable. We avoid protobuf codegen for now (simple tool,
// no protoc required); the methods and types are the contract.
package rpc

import (
	"encoding/json"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

const ProtocolVersion = 1

// Method names.
const (
	MethodPing      = "Ping"
	MethodCreate    = "Create"
	MethodList      = "List"
	MethodGet       = "Get"
	MethodEnter     = "Enter"
	MethodRemove    = "Remove"
	MethodSetPinned = "SetPinned"
	MethodPerdure   = "Perdure"
	MethodGC        = "GC"
	MethodMaintain  = "Maintain"
	MethodAddTag    = "AddTag"
	MethodRemoveTag = "RemoveTag"
	MethodFindByTag = "FindByTag"
	MethodAddNote   = "AddNote"
	MethodNotesFor  = "NotesFor"
)

// Request is one client call.
type Request struct {
	V      int             `json:"v"`
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is one server reply.
type Response struct {
	V      int             `json:"v"`
	ID     int64           `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// --- params / results -------------------------------------------------------

type CreateParams struct {
	Description string `json:"description"`
	Durable     bool   `json:"durable,omitempty"`
}

type IDParams struct {
	ID string `json:"id"`
}

type PerdureParams struct {
	ID    string `json:"id"`
	Root  string `json:"root,omitempty"`
	NoPin bool   `json:"no_pin,omitempty"`
}

type SetPinnedParams struct {
	ID     string `json:"id"`
	Pinned bool   `json:"pinned"`
}

type GCParams struct {
	// MaxAgeNanos is 0 for "all unpinned".
	MaxAgeNanos int64 `json:"max_age_nanos"`
}

type GCResult struct {
	Removed []string `json:"removed"`
}

type MaintainParams struct {
	GCOlderThanNanos int64 `json:"gc_older_than_nanos"`
	SkipGC           bool  `json:"skip_gc,omitempty"`
	SkipOrphanScan   bool  `json:"skip_orphan_scan,omitempty"`
}

type TagParams struct {
	ID  string `json:"id"`
	Tag string `json:"tag"`
}

type FindParams struct {
	Tag string `json:"tag"`
}

type NoteParams struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type WorkspaceDTO struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	Pinned      bool      `json:"pinned,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Notes       []NoteDTO `json:"notes,omitempty"`
}

type NoteDTO struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type ListResult struct {
	Workspaces []WorkspaceDTO `json:"workspaces"`
}

type NotesResult struct {
	Notes []NoteDTO `json:"notes"`
}

type PingResult struct {
	Version string `json:"version"`
}

// --- conversions ------------------------------------------------------------

func WorkspaceToDTO(ws *store.Workspace) WorkspaceDTO {
	if ws == nil {
		return WorkspaceDTO{}
	}
	notes := make([]NoteDTO, len(ws.Notes))
	for i, n := range ws.Notes {
		notes[i] = NoteDTO{ID: n.ID, Body: n.Body, CreatedAt: n.CreatedAt}
	}
	tags := ws.Tags
	if tags == nil {
		tags = []string{}
	}
	return WorkspaceDTO{
		ID:          ws.ID,
		Path:        ws.Path,
		Description: ws.Description,
		CreatedAt:   ws.CreatedAt,
		LastUsed:    ws.LastUsed,
		Pinned:      ws.Pinned,
		Tags:        tags,
		Notes:       notes,
	}
}

func WorkspaceFromDTO(d WorkspaceDTO) *store.Workspace {
	notes := make([]store.Note, len(d.Notes))
	for i, n := range d.Notes {
		notes[i] = store.Note{ID: n.ID, Body: n.Body, CreatedAt: n.CreatedAt}
	}
	return &store.Workspace{
		ID:          d.ID,
		Path:        d.Path,
		Description: d.Description,
		CreatedAt:   d.CreatedAt,
		LastUsed:    d.LastUsed,
		Pinned:      d.Pinned,
		Tags:        d.Tags,
		Notes:       notes,
	}
}

func WorkspacesFromDTO(ds []WorkspaceDTO) []store.Workspace {
	out := make([]store.Workspace, len(ds))
	for i := range ds {
		out[i] = *WorkspaceFromDTO(ds[i])
	}
	return out
}

func NoteFromDTO(n NoteDTO) *store.Note {
	return &store.Note{ID: n.ID, Body: n.Body, CreatedAt: n.CreatedAt}
}

package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

// Handler is what the daemon implements (store-backed).
type Handler struct {
	Store   *store.Store
	Version string
}

// ServeConn handles one client connection (sequential requests per conn).
func (h *Handler) ServeConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	// Workspaces metadata is small; still allow roomy lines.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	w := bufio.NewWriter(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !sc.Scan() {
			return
		}
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = writeResp(w, Response{V: ProtocolVersion, OK: false, Error: "invalid request json"})
			continue
		}
		resp := h.dispatch(ctx, req)
		if err := writeResp(w, resp); err != nil {
			return
		}
	}
}

func writeResp(w *bufio.Writer, resp Response) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

func (h *Handler) dispatch(ctx context.Context, req Request) Response {
	resp := Response{V: ProtocolVersion, ID: req.ID}
	if req.V != 0 && req.V != ProtocolVersion {
		resp.Error = fmt.Sprintf("unsupported protocol version %d", req.V)
		return resp
	}

	var (
		result any
		err    error
	)
	switch req.Method {
	case MethodPing:
		result = PingResult{Version: h.Version}
	case MethodCreate:
		var p CreateParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var ws *store.Workspace
			ws, err = h.Store.CreateOpts(store.CreateOptions{
				Description: p.Description,
				Durable:     p.Durable,
			})
			if err == nil {
				result = WorkspaceToDTO(ws)
			}
		}
	case MethodList:
		var list []store.Workspace
		list, err = h.Store.List()
		if err == nil {
			dtos := make([]WorkspaceDTO, len(list))
			for i := range list {
				dtos[i] = WorkspaceToDTO(&list[i])
			}
			result = ListResult{Workspaces: dtos}
		}
	case MethodGet:
		var p IDParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var ws *store.Workspace
			ws, err = h.Store.Get(p.ID)
			if err == nil {
				// Include tags/notes for show.
				tags, _ := h.Store.TagsFor(p.ID)
				notes, _ := h.Store.NotesFor(p.ID)
				ws.Tags = tags
				ws.Notes = notes
				result = WorkspaceToDTO(ws)
			}
		}
	case MethodEnter:
		var p IDParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var ws *store.Workspace
			ws, err = h.Store.Get(p.ID)
			if err == nil {
				if _, statErr := os.Stat(ws.Path); statErr != nil {
					err = fmt.Errorf("workspace directory missing: %s (try talaria rm %s)", ws.Path, ws.ID)
				} else if err = h.Store.Touch(p.ID); err == nil {
					ws, err = h.Store.Get(p.ID)
					if err == nil {
						result = WorkspaceToDTO(ws)
					}
				}
			}
		}
	case MethodRemove:
		var p IDParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			err = h.Store.Remove(p.ID)
			if err == nil {
				result = map[string]string{"status": "ok"}
			}
		}
	case MethodSetPinned:
		var p SetPinnedParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			err = h.Store.SetPinned(p.ID, p.Pinned)
			if err == nil {
				result = map[string]string{"status": "ok"}
			}
		}
	case MethodPerdure:
		var p PerdureParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var ws *store.Workspace
			ws, err = h.Store.Perdure(p.ID, store.PerdureOptions{
				Root:  p.Root,
				NoPin: p.NoPin,
			})
			if err == nil {
				result = WorkspaceToDTO(ws)
			}
		}
	case MethodGC:
		var p GCParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var removed []string
			removed, err = h.Store.GC(time.Duration(p.MaxAgeNanos))
			if err == nil {
				if removed == nil {
					removed = []string{}
				}
				result = GCResult{Removed: removed}
			}
		}
	case MethodMaintain:
		var p MaintainParams
		if len(req.Params) > 0 {
			err = json.Unmarshal(req.Params, &p)
		}
		if err == nil {
			var mr *store.MaintainResult
			mr, err = h.Store.Maintain(store.MaintainOptions{
				GCOlderThan:    time.Duration(p.GCOlderThanNanos),
				SkipGC:         p.SkipGC,
				SkipOrphanScan: p.SkipOrphanScan,
			})
			if err == nil {
				result = mr
			}
		}
	case MethodAddTag:
		var p TagParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			err = h.Store.AddTag(p.ID, p.Tag)
			if err == nil {
				result = map[string]string{"status": "ok"}
			}
		}
	case MethodRemoveTag:
		var p TagParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			err = h.Store.RemoveTag(p.ID, p.Tag)
			if err == nil {
				result = map[string]string{"status": "ok"}
			}
		}
	case MethodFindByTag:
		var p FindParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var list []store.Workspace
			list, err = h.Store.FindByTag(p.Tag)
			if err == nil {
				dtos := make([]WorkspaceDTO, len(list))
				for i := range list {
					dtos[i] = WorkspaceToDTO(&list[i])
				}
				result = ListResult{Workspaces: dtos}
			}
		}
	case MethodAddNote:
		var p NoteParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var n *store.Note
			n, err = h.Store.AddNote(p.ID, p.Body)
			if err == nil {
				result = NoteDTO{ID: n.ID, Body: n.Body, CreatedAt: n.CreatedAt}
			}
		}
	case MethodNotesFor:
		var p IDParams
		if err = json.Unmarshal(req.Params, &p); err == nil {
			var notes []store.Note
			notes, err = h.Store.NotesFor(p.ID)
			if err == nil {
				dtos := make([]NoteDTO, len(notes))
				for i, n := range notes {
					dtos[i] = NoteDTO{ID: n.ID, Body: n.Body, CreatedAt: n.CreatedAt}
				}
				result = NotesResult{Notes: dtos}
			}
		}
	default:
		err = fmt.Errorf("unknown method %q", req.Method)
	}

	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	raw, mErr := json.Marshal(result)
	if mErr != nil {
		resp.Error = mErr.Error()
		return resp
	}
	resp.OK = true
	resp.Result = raw
	_ = ctx // reserved for future cancel-aware ops
	return resp
}

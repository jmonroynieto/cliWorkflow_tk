package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	regPath := filepath.Join(dir, "workspaces.json")
	st, err := Open(regPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, dir
}

func TestCreateListRemove(t *testing.T) {
	st, dir := testStore(t)

	ws, err := st.Create("mmap probe")
	if err != nil {
		t.Fatal(err)
	}
	if ws.ID == "" || ws.Path == "" {
		t.Fatalf("empty id/path: %+v", ws)
	}
	if _, err := os.Stat(ws.Path); err != nil {
		t.Fatalf("dir missing: %v", err)
	}

	// Registry file is readable JSON.
	raw, err := os.ReadFile(filepath.Join(dir, "workspaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc fileData
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if len(doc.Workspaces) != 1 || doc.Workspaces[0].Description != "mmap probe" {
		t.Fatalf("doc: %+v", doc)
	}

	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Description != "mmap probe" {
		t.Fatalf("list: %+v", list)
	}

	if err := st.Remove(ws.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ws.Path); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone, got %v", err)
	}
	list, _ = st.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestPerdureAndCreateDurable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	t.Setenv("TALARIA_DURABLE_ROOT", filepath.Join(dir, "durable"))
	regPath := filepath.Join(dir, "workspaces.json")
	st, err := Open(regPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ws, err := st.Create("ephemeral")
	if err != nil {
		t.Fatal(err)
	}
	if !paths.UnderRoot(ws.Path, filepath.Join(dir, "ws")) {
		t.Fatalf("expected ephemeral path, got %s", ws.Path)
	}
	// Drop a marker file so we know the tree moved.
	marker := filepath.Join(ws.Path, "marker.txt")
	if err := os.WriteFile(marker, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := st.Perdure(ws.ID, PerdureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(dir, "durable", ws.ID)
	if moved.Path != wantPath {
		t.Fatalf("path: got %s want %s", moved.Path, wantPath)
	}
	if !moved.Pinned {
		t.Fatal("expected pin after perdure")
	}
	if _, err := os.Stat(filepath.Join(wantPath, "marker.txt")); err != nil {
		t.Fatalf("marker missing after move: %v", err)
	}
	if _, err := os.Stat(ws.Path); !os.IsNotExist(err) {
		t.Fatalf("old path should be gone: %s", ws.Path)
	}

	// Idempotent.
	again, err := st.Perdure(ws.ID, PerdureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Path != wantPath {
		t.Fatalf("idempotent path: %s", again.Path)
	}

	// Create already durable.
	d, err := st.CreateOpts(CreateOptions{Description: "long job", Durable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Pinned {
		t.Fatal("durable create should pin")
	}
	if d.Path != filepath.Join(dir, "durable", d.ID) {
		t.Fatalf("durable path: %s", d.Path)
	}
}

func TestPinAndGC(t *testing.T) {
	st, _ := testStore(t)

	a, err := st.Create("keep")
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.Create("drop")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPinned(a.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.TouchAt(b.ID, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	removed, err := st.GC(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != b.ID {
		t.Fatalf("removed: %v", removed)
	}

	if _, err := st.Get(a.ID); err != nil {
		t.Fatalf("pinned should survive: %v", err)
	}
	if _, err := st.Get(b.ID); err == nil {
		t.Fatal("unpinned old should be gone")
	}
}

func TestTagsAndNotes(t *testing.T) {
	st, _ := testStore(t)
	ws, err := st.Create("tagged")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddTag(ws.ID, "zig"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddTag(ws.ID, "parser"); err != nil {
		t.Fatal(err)
	}
	// Idempotent re-tag.
	if err := st.AddTag(ws.ID, "zig"); err != nil {
		t.Fatal(err)
	}
	found, err := st.FindByTag("zig")
	if err != nil || len(found) != 1 {
		t.Fatalf("find: %v %v", found, err)
	}
	n, err := st.AddNote(ws.ID, "benchmark mmap")
	if err != nil || n.Body != "benchmark mmap" || n.ID != 1 {
		t.Fatalf("note: %+v %v", n, err)
	}
	notes, err := st.NotesFor(ws.ID)
	if err != nil || len(notes) != 1 {
		t.Fatalf("notes: %v %v", notes, err)
	}

	// Reload from disk — must release the exclusive lock first.
	path := st.Path()
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	ws2, err := st2.Get(ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws2.Tags) != 2 || len(ws2.Notes) != 1 {
		t.Fatalf("reload: tags=%v notes=%v", ws2.Tags, ws2.Notes)
	}
}

func TestMaintainReconcileMissing(t *testing.T) {
	st, _ := testStore(t)
	ws, err := st.Create("will vanish")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(ws.Path); err != nil {
		t.Fatal(err)
	}
	// Plant an orphan directory not in the registry.
	orphan := filepath.Join(os.Getenv("TALARIA_WORKSPACE_ROOT"), "orphan1")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := st.Maintain(MaintainOptions{SkipGC: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Missing) != 1 || res.Missing[0] != ws.ID {
		t.Fatalf("missing: %+v", res.Missing)
	}
	if res.Kept != 0 {
		t.Fatalf("kept=%d", res.Kept)
	}
	if len(res.Orphans) != 1 || res.Orphans[0] != orphan {
		t.Fatalf("orphans: %+v", res.Orphans)
	}
	if _, err := st.Get(ws.ID); err == nil {
		t.Fatal("expected registry entry gone")
	}
}

func TestOpenMissingCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspaces.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); err != nil {
		t.Fatalf("expected lock file: %v", err)
	}
}

func TestConcurrentCreates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	regPath := filepath.Join(dir, "workspaces.json")

	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st, err := Open(regPath)
			if err != nil {
				errs <- err
				return
			}
			defer st.Close()
			if _, err := st.Create("concurrent"); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	st, err := Open(regPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != n {
		t.Fatalf("want %d workspaces after concurrent creates, got %d", n, len(list))
	}

	// Registry must still be valid JSON (no torn writes).
	raw, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc fileData
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("corrupt registry after concurrent writes: %v\n%s", err, raw)
	}
}

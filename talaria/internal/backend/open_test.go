package backend_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/backend"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/daemon"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

func TestOpenFallsBackToLocal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", dir)
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	t.Setenv("TALARIA_SOCKET", filepath.Join(dir, "missing.sock"))
	t.Setenv("TALARIA_FORCE_LOCAL", "")
	t.Setenv("TALARIA_FORCE_DAEMON", "")

	b, err := backend.Open(context.Background(), backend.OpenOptions{Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if b.Mode() != backend.ModeLocal {
		t.Fatalf("mode=%s want local", b.Mode())
	}
	ws, err := b.Create(context.Background(), store.CreateOptions{Description: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if ws.ID == "" {
		t.Fatal("empty id")
	}
}

func TestOpenUsesDaemonWhenUp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", dir)
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	sock := filepath.Join(dir, "talaria.sock")
	t.Setenv("TALARIA_SOCKET", sock)
	reg := filepath.Join(dir, "workspaces.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Run(ctx, daemon.Config{
			Version:      "test",
			SocketPath:   sock,
			RegistryPath: reg,
			GCInterval:   0,
		})
	}()

	// Wait until socket is up.
	deadline := time.Now().Add(3 * time.Second)
	var b backend.Backend
	var err error
	for time.Now().Before(deadline) {
		b, err = backend.Open(context.Background(), backend.OpenOptions{
			ForceDaemon: true,
			DialTimeout: 100 * time.Millisecond,
			Quiet:       true,
		})
		if err == nil {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("daemon never became ready: %v", err)
	}
	defer b.Close()

	if b.Mode() != backend.ModeDaemon {
		t.Fatalf("mode=%s want daemon", b.Mode())
	}
	ws, err := b.Create(context.Background(), store.CreateOptions{Description: "via daemon"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := b.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != ws.ID {
		t.Fatalf("list: %+v", list)
	}

	// Force local while daemon holds lock should time out / fail.
	_, err = backend.Open(context.Background(), backend.OpenOptions{
		ForceLocal: true,
		Quiet:      true,
	})
	if err == nil {
		t.Fatal("expected local open to fail while daemon holds lock")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit")
	}

	// After daemon exit, local works again.
	b2, err := backend.Open(context.Background(), backend.OpenOptions{
		ForceLocal: true,
		Quiet:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	list, err = b2.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("registry should persist: %d", len(list))
	}

	_ = paths.SocketPath // silence if unused in future
}

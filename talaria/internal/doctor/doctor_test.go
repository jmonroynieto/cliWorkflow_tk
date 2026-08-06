package doctor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/store"
)

func TestRunLocalHealthy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	t.Setenv("TALARIA_SOCKET", filepath.Join(dir, "talaria.sock"))

	// Materialize a registry so the parse check is ok (not just "not created").
	reg := filepath.Join(dir, "data", "workspaces.json")
	st, err := store.Open(reg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create("doctor test"); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	r, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !r.Healthy() {
		t.Fatalf("expected healthy:\n%s", Format(r))
	}

	// Socket should be info (not running), not fail.
	var socket, registry, root, lock Check
	for _, c := range r.Checks {
		switch c.Name {
		case "socket":
			socket = c
		case "registry":
			registry = c
		case "workspace-root":
			root = c
		case "lock":
			lock = c
		}
	}
	if socket.Status != StatusInfo && socket.Status != StatusOK {
		t.Fatalf("socket: %+v", socket)
	}
	if registry.Status != StatusOK {
		t.Fatalf("registry: %+v", registry)
	}
	if root.Status != StatusOK {
		t.Fatalf("workspace-root: %+v", root)
	}
	// Durable root is also checked.
	var durable Check
	for _, c := range r.Checks {
		if c.Name == "durable-root" {
			durable = c
		}
	}
	// Under t.TempDir() the durable root is on volatile storage → warn is ok.
	if durable.Status != StatusOK && durable.Status != StatusWarn {
		t.Fatalf("durable-root: %+v", durable)
	}
	if lock.Status != StatusOK && lock.Status != StatusInfo {
		t.Fatalf("lock: %+v", lock)
	}
}

func TestRegistryUnparseable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	t.Setenv("TALARIA_SOCKET", filepath.Join(dir, "talaria.sock"))

	reg := filepath.Join(dir, "data", "workspaces.json")
	if err := os.MkdirAll(filepath.Dir(reg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, []byte("not-json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Healthy() {
		t.Fatalf("expected unhealthy for bad JSON:\n%s", Format(r))
	}
	var found bool
	for _, c := range r.Checks {
		if c.Name == "registry" && c.Status == StatusFail {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected registry fail:\n%s", Format(r))
	}
}

func TestParsePIDTokens(t *testing.T) {
	got := parsePIDTokens("path:  1234c  56m  notpid  1234")
	if len(got) != 2 || got[0] != 1234 || got[1] != 56 {
		t.Fatalf("got %v", got)
	}
}

func TestFormatIncludesPaths(t *testing.T) {
	r := Report{
		Paths: Paths{Socket: "/tmp/s", Lock: "/tmp/l"},
		Checks: []Check{
			{Name: "socket", Status: StatusOK, Detail: "reachable"},
		},
	}
	s := Format(r)
	if !strings.Contains(s, "/tmp/s") || !strings.Contains(s, "[ok]") {
		t.Fatalf("format:\n%s", s)
	}
}

func TestLockHeldByUs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("TALARIA_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	t.Setenv("TALARIA_SOCKET", filepath.Join(dir, "talaria.sock"))

	reg := filepath.Join(dir, "data", "workspaces.json")
	st, err := store.Open(reg)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Write minimal valid JSON under the lock (store already has it).
	_ = json.RawMessage{}

	r, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var lock Check
	for _, c := range r.Checks {
		if c.Name == "lock" {
			lock = c
		}
	}
	if lock.Status == StatusFail {
		t.Fatalf("lock fail: %+v", lock)
	}
	// While this process holds the store lock, probe should see held.
	held, err := probeLockHeld(reg + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("expected lock held while store open")
	}
}

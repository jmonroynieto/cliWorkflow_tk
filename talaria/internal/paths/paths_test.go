package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPIDFileBeside(t *testing.T) {
	cases := []struct {
		sock, want string
	}{
		{"/run/user/1000/talaria.sock", "/run/user/1000/talaria.pid"},
		{"/tmp/custom.sock", "/tmp/custom.pid"},
		{"/tmp/nosuffix", "/tmp/nosuffix.pid"},
	}
	for _, tc := range cases {
		got := PIDFileBeside(tc.sock)
		if got != tc.want {
			t.Errorf("PIDFileBeside(%q)=%q want %q", tc.sock, got, tc.want)
		}
	}
}

func TestLockPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", dir)
	got, err := LockPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "workspaces.json.lock")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDurableRootAndUnderRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TALARIA_DATA_DIR", dir)
	t.Setenv("TALARIA_DURABLE_ROOT", "")
	// clear durable override if set by parent
	_ = os.Unsetenv("TALARIA_DURABLE_ROOT")

	got, err := DurableRoot()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "workspaces")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !UnderRoot(filepath.Join(want, "abcd"), want) {
		t.Fatal("expected under durable root")
	}
	if UnderRoot("/tmp/x", want) {
		t.Fatal("tmp should not be under durable")
	}
	if !IsVolatilePath(filepath.Join(os.TempDir(), "talaria", "x")) {
		t.Fatal("expected volatile under TempDir")
	}
}

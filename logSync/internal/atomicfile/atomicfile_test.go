package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWrite_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "new.md")
	if err := Write(dest, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != defaultMode {
		t.Errorf("expected default mode %o, got %o", defaultMode, info.Mode().Perm())
	}
}

func TestWrite_PreservesExistingMode(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(dest, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(dest, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected preserved mode 0600, got %o", info.Mode().Perm())
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "new" {
		t.Errorf("got %q", got)
	}
}

func TestWrite_NoTempFileLeftBehindOnSuccess(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "f.md")
	if err := Write(dest, []byte("x")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "f.md" {
		t.Errorf("expected only f.md in %s, got %v", dir, entries)
	}
}

func TestCopyPreservingTimestamp(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.md")
	dst := filepath.Join(dir, "dst.md")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(src, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	if err := CopyPreservingTimestamp(src, dst); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "content" {
		t.Errorf("got %q", got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("expected mtime %v, got %v", mtime, info.ModTime())
	}
}

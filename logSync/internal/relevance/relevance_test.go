package relevance

import (
	"os"
	"path/filepath"
	"testing"
)

// These cases are ported 1:1 from the bash tool's own test suite, which is
// the conformance oracle for this package: these exact fixtures must be
// classified the same way the tool this replaces classified them.
func TestRelevant_BashParity(t *testing.T) {
	dateUpdated := []string{ObsidianDateUpdatedPattern}

	note := func(dir, name, dateUpdatedValue string, body ...string) string {
		content := "---\ndateUpdated: " + dateUpdatedValue + "\n---\n"
		for _, l := range body {
			content += l + "\n"
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	plain := func(dir, name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("dateUpdated-only difference is irrelevant", func(t *testing.T) {
		dir := t.TempDir()
		a := note(dir, "a-local.md", "2026-08-01", "- shared")
		b := note(dir, "a-phone.md", "2026-08-09", "- shared")
		assertRelevant(t, a, b, dateUpdated, false)
	})

	t.Run("added bare '---' IS relevant (old regex swallowed it)", func(t *testing.T) {
		dir := t.TempDir()
		a := plain(dir, "b-local.md", "body\n")
		b := plain(dir, "b-phone.md", "---\nbody\n")
		assertRelevant(t, a, b, dateUpdated, true)
	})

	for _, x := range []string{"--", "++", "-+"} {
		t.Run("added '"+x+"' IS relevant", func(t *testing.T) {
			dir := t.TempDir()
			a := plain(dir, "c-local.md", "body\n")
			b := plain(dir, "c-phone.md", x+"\nbody\n")
			assertRelevant(t, a, b, dateUpdated, true)
		})
	}

	t.Run("identical files are irrelevant", func(t *testing.T) {
		dir := t.TempDir()
		a := plain(dir, "d-local.md", "body\n")
		b := plain(dir, "d-phone.md", "body\n")
		assertRelevant(t, a, b, dateUpdated, false)
	})

	t.Run("added bullet IS relevant", func(t *testing.T) {
		dir := t.TempDir()
		a := plain(dir, "e-local.md", "body\n")
		b := plain(dir, "e-phone.md", "body\n- new bullet\n")
		assertRelevant(t, a, b, dateUpdated, true)
	})
}

func TestRelevant_NoIgnorePatterns(t *testing.T) {
	// With no ignore patterns configured (the generic, non-Obsidian default),
	// any byte difference is relevant — including a dateUpdated-only edit.
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	os.WriteFile(a, []byte("dateUpdated: 2026-08-01\nbody\n"), 0o644)
	os.WriteFile(b, []byte("dateUpdated: 2026-08-09\nbody\n"), 0o644)
	assertRelevant(t, a, b, nil, true)
}

func assertRelevant(t *testing.T, a, b string, ignore []string, want bool) {
	t.Helper()
	got, err := Relevant(a, b, ignore)
	if err != nil {
		t.Fatalf("Relevant(%s, %s): %v", a, b, err)
	}
	if got != want {
		t.Errorf("Relevant(%s, %s) = %v, want %v", a, b, got, want)
	}
}

// `diff -u` on two binaries prints one line — "Binary files a and b
// differ" — which the header skip then discards, leaving no +/- lines and
// so the verdict "not relevant". Every changed font, image and PDF in a
// vault was therefore classified as unchanged and never synced.
func TestRelevant_ChangedBinaryContentIsRelevant(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	if err := os.WriteFile(a, []byte{0x89, 'P', 'N', 'G', 0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte{0x89, 'P', 'N', 'G', 0x00, 0x09, 0x09}, 0o644); err != nil {
		t.Fatal(err)
	}
	relevant, err := Relevant(a, b, []string{ObsidianDateUpdatedPattern})
	if err != nil {
		t.Fatal(err)
	}
	if !relevant {
		t.Error("two binaries with different bytes must count as differing")
	}
}

func TestRelevant_IdenticalBinariesAreNotRelevant(t *testing.T) {
	dir := t.TempDir()
	content := []byte{0x00, 0x01, 0x02, 0x03}
	a := filepath.Join(dir, "a.bin")
	b := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(a, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, content, 0o644); err != nil {
		t.Fatal(err)
	}
	relevant, err := Relevant(a, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if relevant {
		t.Error("identical bytes are never a relevant difference")
	}
}

// An ignore pattern must not be able to talk a binary difference away.
func TestRelevant_IgnorePatternsDoNotApplyToBinaries(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.bin")
	b := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(a, []byte("dateUpdated: 1\x00padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("dateUpdated: 2\x00padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	relevant, err := Relevant(a, b, []string{ObsidianDateUpdatedPattern})
	if err != nil {
		t.Fatal(err)
	}
	if !relevant {
		t.Error("a line pattern must not suppress a difference in non-text content")
	}
}

func TestIsBinary(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string][]byte{
		"note.md":  []byte("---\ntitle: t\n---\n\nplain text\n"),
		"empty.md": {},
		"utf8.md":  []byte("café — résumé\n"),
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := IsBinary(p)
		if err != nil {
			t.Fatal(err)
		}
		if got {
			t.Errorf("%s: text was called binary", name)
		}
	}
	p := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(p, append([]byte("head"), 0x00, 0xff), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := IsBinary(p)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("a NUL byte means this is not text")
	}
}

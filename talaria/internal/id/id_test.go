package id

import "testing"

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		s, err := New(4)
		if err != nil {
			t.Fatal(err)
		}
		if len(s) != 4 {
			t.Fatalf("len=%d want 4: %q", len(s), s)
		}
		for _, c := range s {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				t.Fatalf("bad char %q in %q", c, s)
			}
		}
		seen[s] = true
	}
	if len(seen) < 40 {
		t.Fatalf("expected high uniqueness, got %d unique", len(seen))
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseContainerdRoot(t *testing.T) {
	cases := []struct {
		name     string
		contents string // empty means: do not create the file
		want     string
		exists   bool
	}{
		{"missing file falls back to default", "", defaultContainerdRoot, false},
		{"plain root key", "version = 3\nroot = \"/CONDA/containerd\"\n", "/CONDA/containerd", true},
		{"root after comments and blanks", "# generated\n\nversion = 3\n\nroot = \"/data/cd\"\n", "/data/cd", true},
		{"no root key keeps default", "version = 3\nstate = \"/run/containerd\"\n", defaultContainerdRoot, true},
		{"root inside a plugin table is ignored", "version = 3\n[plugins.\"io.containerd.x\"]\n  root = \"/wrong\"\n", defaultContainerdRoot, true},
		{"top-level root wins over a later table", "root = \"/right\"\n[plugins.\"x\"]\n  root = \"/wrong\"\n", "/right", true},
		{"spacing around the equals sign", "root   =    \"/spaced\"\n", "/spaced", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tc.contents != "" {
				if err := os.WriteFile(path, []byte(tc.contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, exists, err := parseContainerdRoot(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("root = %q, want %q", got, tc.want)
			}
			if exists != tc.exists {
				t.Errorf("exists = %t, want %t", exists, tc.exists)
			}
		})
	}
}

func TestUsesContainerdSnapshotter(t *testing.T) {
	if !(containerdState{driverType: "io.containerd.snapshotter.v1"}).usesContainerdSnapshotter() {
		t.Error("snapshotter driver-type not recognised")
	}
	if (containerdState{driverType: ""}).usesContainerdSnapshotter() {
		t.Error("empty driver-type treated as snapshotter")
	}
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name   string
		st     containerdState
		prefix string
	}{
		{"off root partition", containerdState{rootOnSlash: false}, "OK"},
		{"snapshotter on root partition", containerdState{rootOnSlash: true, driverType: "io.containerd.snapshotter.v1"}, "PROBLEM"},
		{"graph driver on root partition", containerdState{rootOnSlash: true, driverType: ""}, "NOTE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.st.diagnose(); len(got) < len(tc.prefix) || got[:len(tc.prefix)] != tc.prefix {
				t.Errorf("diagnose() = %q, want prefix %q", got, tc.prefix)
			}
		})
	}
}

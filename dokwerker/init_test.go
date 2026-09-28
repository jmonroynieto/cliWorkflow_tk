package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v3"
)

// TestInitOverwriteGuard checks that init leaves existing Docker files alone
// unless --force is given.
func TestInitOverwriteGuard(t *testing.T) {
	skriptujo := t.TempDir()
	boilerplate := filepath.Join(skriptujo, "boilerplate", "docker")
	if err := os.MkdirAll(boilerplate, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"maths-docker-compose.yml": "template compose\n",
		"maths-dockfile":           "template dockerfile\n",
	} {
		if err := os.WriteFile(filepath.Join(boilerplate, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("scriptLoc", skriptujo)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("Dockerfile", []byte("project dockerfile\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// app.Run would os.Exit on the guard's error, so run InitAction from a
	// minimal command whose error handler returns instead.
	runInit := func(args ...string) error {
		cmd := &cli.Command{
			Name: "dokwerker",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stack", Value: "maths"},
				&cli.BoolFlag{Name: "force"},
			},
			ExitErrHandler: func(context.Context, *cli.Command, error) {},
			Action:         InitAction,
		}
		return cmd.Run(context.Background(), append([]string{"dokwerker"}, args...))
	}

	readDockerfile := func() string {
		b, err := os.ReadFile("Dockerfile")
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if err := runInit(); err == nil {
		t.Fatal("init without --force succeeded over an existing Dockerfile")
	}
	if got := readDockerfile(); got != "project dockerfile\n" {
		t.Fatalf("Dockerfile changed without --force: %q", got)
	}
	if _, err := os.Stat("docker-compose.yml"); !os.IsNotExist(err) {
		t.Fatalf("docker-compose.yml created by a refused init: %v", err)
	}

	if err := runInit("--force"); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	if got := readDockerfile(); got != "template dockerfile\n" {
		t.Fatalf("Dockerfile not replaced by --force: %q", got)
	}
}

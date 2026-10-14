package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIOutputAndClipboard(t *testing.T) {
	for _, bare := range []bool{false, true} {
		file := filepath.Join(t.TempDir(), "dois")
		os.WriteFile(file, []byte("10.1234/a\n\n10.1234/b"), 0600)
		args := []string{"--oneshot", file}
		if bare {
			args = append(args, "--bare")
		}
		var out, diag bytes.Buffer
		var copied []string
		count := 0
		code := runWith(args, strings.NewReader(""), &out, &diag, func(context.Context, string) (string, error) { count++; return "@article{k, title={T}}", nil }, func(s string) error { copied = append(copied, s); return errors.New("no display") })
		if code != 0 || count != 2 || len(copied) != 2 {
			t.Fatal(code, count, copied)
		}
		if strings.Contains(out.String(), "```") == bare || strings.Contains(out.String(), "\x1b") {
			t.Fatal(out.String())
		}
		if !strings.Contains(diag.String(), "Clipboard unavailable") {
			t.Fatal(diag.String())
		}
	}
}
func TestInteractiveAndFailure(t *testing.T) {
	var out, diag bytes.Buffer
	count := 0
	code := runWith([]string{"--no-clipboard"}, strings.NewReader("10.1234/a\n\n10.1234/b\n"), &out, &diag, func(context.Context, string) (string, error) { count++; return "", errors.New("lookup failed") }, func(string) error { t.Fatal("clipboard called"); return nil })
	if code != 1 || count != 1 || out.Len() != 0 {
		t.Fatal(code, count, out.String())
	}
}

package doi2bib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for _, in := range []string{" 10.1234/abc ", "https://doi.org/10.1234/abc", `"10.1234°abc"`, "10.1234/a b c", "10.1234/ab\r\n\tc", "https://doi.org/10. 1234/a\u00a0b\u2003c"} {
		got, e := Normalize(in)
		if e != nil || got != "10.1234/abc" {
			t.Fatal(got, e)
		}
	}
	for _, in := range []string{"", " \t\n", "10.12/abc", "10.1234/", "10.1234/ \t\n"} {
		if _, e := Normalize(in); e == nil {
			t.Fatal(in)
		}
	}
}
func TestLookup(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		fail       bool
	}{{"ok", "@article{k, title={T}}", 200, false}, {"html", "<html>error</html>", 200, true}, {"status", "missing", 404, true}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept") != "application/x-bibtex" {
					t.Error("missing accept")
				}
				if r.URL.Path == "/10.1234/abc" {
					http.Redirect(w, r, "/metadata", http.StatusFound)
					return
				}
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := Client{BaseURL: srv.URL, Format: func(_ context.Context, s string) (string, error) { return s + " formatted", nil }}
			got, e := c.Lookup(context.Background(), "10.1234/abc")
			if (e != nil) != tc.fail {
				t.Fatal(got, e)
			}
			if !tc.fail && !strings.HasSuffix(got, " formatted") {
				t.Fatal(got)
			}
		})
	}
}
func TestFormatterPipeline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	os.WriteFile(filepath.Join(dir, "bibtex-tidy"), []byte("#!/bin/sh\ncat\nprintf '\\n%% tidy\\n'\n"), 0700)
	os.WriteFile(filepath.Join(dir, "bibstomp"), []byte("#!/bin/sh\n[ \"$1\" = stomp ] || exit 3\ncat\n"), 0700)
	out, e := FormatExternal(context.Background(), "@article{k, title={T}}")
	if e != nil || !strings.Contains(out, "tidy") {
		t.Fatal(out, e)
	}
	os.WriteFile(filepath.Join(dir, "bibstomp"), []byte("#!/bin/sh\necho broken >&2\nexit 4\n"), 0700)
	if _, e = FormatExternal(context.Background(), "@article{k, title={T}}"); e == nil || !strings.Contains(e.Error(), "broken") {
		t.Fatal(e)
	}
}
func TestCancelledLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := (Client{}).Lookup(ctx, "10.1234/abc")
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestLookupWithoutFormatters(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "@article{k, title={T}}") }))
	defer srv.Close()
	var warnings []string
	got, err := (Client{BaseURL: srv.URL, OnWarning: func(s string) { warnings = append(warnings, s) }}).Lookup(context.Background(), "10.1234/abc")
	want, warningCount := "@article{Anon_0000, title={T}}", 1
	if err != nil || got != want || len(warnings) != warningCount {
		t.Fatal(got, err, warnings)
	}
}
func TestAvailableFormatterStillRunsAndFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	path := filepath.Join(dir, "bibtex-tidy")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '@article{formatted, author={Smith, Alice}, year={2024}, title={T}}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var warnings []string
	got, err := formatAvailable(context.Background(), "@article{k, title={T}}", func(s string) { warnings = append(warnings, s) })
	wantKey, warningCount := "Smith_2024", 0
	if err != nil || !strings.Contains(got, wantKey) || len(warnings) != warningCount {
		t.Fatal(got, err, warnings)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho broken >&2\nexit 4\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := formatAvailable(context.Background(), "@article{k, title={T}}", nil); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := formatAvailable(ctx, "@article{k, title={T}}", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestLibraryStompIgnoresPATH(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "bibstomp"), []byte("#!/bin/sh\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	got, err := formatAvailable(context.Background(), "@article{original, author={García, José}, year={2024}, title={T}}", nil)
	if err != nil || !strings.HasPrefix(got, "@article{Garcia_2024,") {
		t.Fatal(got, err)
	}
}

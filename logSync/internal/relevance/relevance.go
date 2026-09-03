// Package relevance decides whether a byte-level difference between two
// files is worth merging, or is noise that should be ignored — e.g. a
// frontmatter timestamp an editor rewrites on every touch. This generalizes
// the bash tool's hardcoded `dateUpdated:`-only rule into a configurable
// set of ignore patterns; ObsidianDateUpdatedPattern below reproduces that
// original behavior as a preset.
package relevance

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
)

// ObsidianDateUpdatedPattern reproduces the original rule: a diff
// line touching only the `dateUpdated:` frontmatter field is not relevant.
// It matches the full diff line including its leading '+'/'-'.
const ObsidianDateUpdatedPattern = `^[+-]dateUpdated:`

// Relevant reports whether files a and b differ in a way not covered by
// ignorePatterns (each matched against a full unified-diff content line,
// leading '+'/'-' included). Byte-identical files are never relevant.
//
// This shells out to the system `diff -u` rather than reimplementing a line
// differ, for the same reason internal/merge shells out to `git
// merge-file`: it's local-only (no per-call cost worth avoiding, unlike the
// phone-side round trips that actually cost anything), and it keeps this
// package's notion of "a diff line" identical to the bash tool it replaces,
// so that tool's tests remain a usable conformance oracle.
func Relevant(a, b string, ignorePatterns []string) (bool, error) {
	identical, err := filesIdentical(a, b)
	if err != nil {
		return false, err
	}
	if identical {
		return false, nil
	}

	// A line diff cannot judge content that is not lines. `diff -u` answers
	// "Binary files a and b differ" — a single line, which the header skip
	// below then discards, leaving no +/- lines and therefore the verdict
	// "not relevant". A changed font, image or PDF was silently classified
	// as unchanged and never synced. The bytes already differ by the time
	// we get here, and for content like this every byte is the content.
	for _, p := range []string{a, b} {
		binary, err := IsBinary(p)
		if err != nil {
			return false, err
		}
		if binary {
			return true, nil
		}
	}

	compiled := make([]*regexp.Regexp, 0, len(ignorePatterns))
	for _, p := range ignorePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return false, fmt.Errorf("invalid ignore pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}

	out, err := exec.Command("diff", "-u", a, b).Output()
	if err != nil {
		// diff exits 1 when files differ — that's expected here since we
		// already know they're not byte-identical. Only a real execution
		// failure (exit >1, or not an ExitError at all) is an error.
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() > 1 {
			return false, fmt.Errorf("diff -u %s %s: %w", a, b, err)
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo <= 2 {
			// `--- a` / `+++ b` unified-diff headers, dropped positionally
			// The original `^[+-]{3}` pattern accidentally matched content
			// lines like a bare `---`, which is the first line of every
			// piece of YAML frontmatter in the vault; dropping the headers
			// by position instead of by pattern fixed it.
			continue
		}
		line := scanner.Text()
		if len(line) == 0 || (line[0] != '+' && line[0] != '-') {
			continue
		}
		ignored := false
		for _, re := range compiled {
			if re.MatchString(line) {
				ignored = true
				break
			}
		}
		if !ignored {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading diff output: %w", err)
	}
	return false, nil
}

// IsBinary uses git's own heuristic: a NUL byte anywhere in the first 8000
// bytes means this is not text.
func IsBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	return bytes.IndexByte(buf[:n], 0) >= 0, nil
}

func filesIdentical(a, b string) (bool, error) {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", a, err)
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", b, err)
	}
	return bytes.Equal(ab, bb), nil
}

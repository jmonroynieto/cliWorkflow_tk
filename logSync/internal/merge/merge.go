// Package merge implements logSync's union-of-lines merge, the same
// operator the bash tool used: idempotent and unital, with the empty file
// as its identity, but on ordered lists of possibly-repeated lines neither
// commutative nor associative.
//
// The bash tool always ran this against an empty base (`git merge-file -p
// --union dest /dev/null src`), which is exactly what makes it
// non-associative and caps convergence at two replicas. Where a git
// tracking repo has a real committed ancestor for a path, this package uses
// it as the merge base instead — a genuine 3-way merge, no new storage
// needed, the tracking repo already has the history. Untracked paths fall
// back to the original empty-base behavior unchanged.
package merge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/atomicfile"
)

// Result reports what git merge-file did.
type Result struct {
	Merged    []byte
	Conflicts int  // 0 means clean
	UsedBase  bool // true if a real ancestor was used, false if empty-base union
}

// Ancestor looks up path's content as of HEAD in repoRoot, the git worktree
// that tracks it. relPath must be relative to repoRoot itself, not to
// whichever subdirectory the caller is syncing (see gitRel in sync.go).
// found is false (with a nil error) when repoRoot has no commit touching
// path — the caller should fall back to an empty-base union merge in that
// case, exactly as the bash tool always did.
//
// This runs `git cat-file blob`, not `git show`. `git show HEAD:<path>`
// exits 0 and prints the whole commit object when <path> contains a `[`,
// because git parses the wildcard as a pathspec rather than a missing file
// — so an untracked note named e.g. "[draft] plan.md" came back as
// found=true with a commit dump standing in for its content. `cat-file
// blob` refuses the same input with the usual exit 128.
func Ancestor(repoRoot, relPath string) (content []byte, found bool, err error) {
	cmd := exec.Command("git", "-C", repoRoot, "cat-file", "blob", "HEAD:"+filepath.ToSlash(relPath))
	out, err := cmd.Output()
	if err == nil {
		return out, true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		// git cat-file exits 128 (fatal) for "path does not exist in HEAD",
		// for a path that names a directory rather than a blob, and for "no
		// such ref" (empty repo) alike; treat all three as "no ancestor"
		// rather than trying to distinguish by scraping stderr text.
		if exitErr.ExitCode() == 128 {
			return nil, false, nil
		}
	}
	return nil, false, fmt.Errorf("git cat-file blob HEAD:%s in %s: %w", relPath, repoRoot, err)
}

// Into merges local and phone, using ancestor as the merge base (pass nil,
// false for the original empty-base union), and atomically writes the
// result to dest preserving dest's existing file mode. conflicts is the
// number of unresolved hunks git merge-file reports; with --union there
// never are any (every hunk is auto-resolved by keeping both sides), so
// conflicts > 0 here would indicate a git behavior change worth noticing,
// not a normal outcome to handle.
func Into(localPath string, ancestor []byte, hasAncestor bool, phonePath, dest string) (Result, error) {
	baseArg := os.DevNull
	if hasAncestor {
		tmp, err := os.CreateTemp("", "logSync-ancestor-*")
		if err != nil {
			return Result{}, fmt.Errorf("create ancestor temp file: %w", err)
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.Write(ancestor); err != nil {
			tmp.Close()
			return Result{}, fmt.Errorf("write ancestor temp file: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return Result{}, fmt.Errorf("close ancestor temp file: %w", err)
		}
		baseArg = tmp.Name()
	}

	cmd := exec.Command("git", "merge-file", "-p", "--union", localPath, baseArg, phonePath)
	out, err := cmd.Output()
	rc := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		rc = exitErr.ExitCode()
	} else if err != nil {
		return Result{}, fmt.Errorf("git merge-file %s %s %s: %w", localPath, baseArg, phonePath, err)
	}
	// git merge-file reports the conflict count as its exit status; only
	// >127 (killed by signal, or a real invocation error) is an actual
	// failure — mirrors the bash merge helper's `((rc > 127))` check.
	if rc > 127 {
		return Result{}, fmt.Errorf("git merge-file failed on %s (rc=%d)", localPath, rc)
	}

	if err := atomicfile.Write(dest, out); err != nil {
		return Result{}, err
	}

	return Result{Merged: out, Conflicts: rc, UsedBase: hasAncestor}, nil
}

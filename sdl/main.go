// sdl (safeDelink) removes files only after confirming another hard link
// to the same data will survive the removal. It refuses to remove a file
// whose link count is 1, since that would destroy the only reference to
// the underlying data. Directories are never removed by sdl -- only the
// regular files found within them (see -r).
package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	Version  string
	Revision = "0"
	CommitId string
)

type options struct {
	recursive   bool
	verbose     bool
	dryRun      bool
	force       bool
	xdev        bool
	showLinks   bool
	interactive bool
	showVersion bool
}

func main() {
	if len(os.Args) < 2  {
		flag.Usage()
		os.Exit(1)
	} 
	opts := options{}

	flag.BoolVar(&opts.recursive, "r", false, "recurse into directories, removing only regular files (never the directories themselves)")
	flag.BoolVar(&opts.recursive, "R", false, "same as -r")
	flag.BoolVar(&opts.recursive, "recursive", false, "same as -r")
	flag.BoolVar(&opts.verbose, "v", false, "explain what is being done")
	flag.BoolVar(&opts.verbose, "verbose", false, "same as -v")
	flag.BoolVar(&opts.dryRun, "n", false, "show what would be removed without removing anything")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "same as -n")
	flag.BoolVar(&opts.force, "f", false, "ignore nonexistent files, never prompt (does NOT bypass the link-count safety check)")
	flag.BoolVar(&opts.force, "force", false, "same as -f")
	flag.BoolVar(&opts.xdev, "xdev", false, "when recursing, never cross into a different filesystem")
	flag.BoolVar(&opts.showLinks, "show-links", false, "before removing a file, print the other paths hard-linked to the same data")
	flag.BoolVar(&opts.interactive, "i", false, "prompt before every removal")
	flag.BoolVar(&opts.interactive, "interactive", false, "same as -i")
	flag.BoolVar(&opts.showVersion, "version", false, "print version information and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: sdl [options] FILE...

sdl removes a file only after confirming its hard-link count is greater
than 1, i.e. that another name for the same data will remain after the
removal. If a file's link count is 1, sdl refuses to remove it, since
that would be the last reference to the data.

Options:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Notes:
  * sdl never removes directories, even with -r. It only removes the
    regular files found while recursing; directory structure is left
    in place.
  * Hard links only ever span a single filesystem, so -r treats --xdev
    as implied guidance, not a hard requirement; pass --xdev explicitly
    to enforce it (matches find -xdev).
  * Symlinks are always safe to remove (removing a symlink never
    touches the data it points to) and are removed unconditionally.
`)
	}

	flag.Parse()
	args := flag.Args()

    if opts.showVersion {
		fmt.Printf("%s.%s (%s)\n", Version, Revision, CommitId)
		os.Exit(0)
	}

	exitCode := 0
	for _, path := range args {
		if err := processArg(path, &opts); err != nil {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/jmonroynieto/cliWorkflow_tk/doi2bib/bibstomp"
)

const usage = `bibstomp -- flatten diacritics in BibTeX and remint cite keys

usage:
  bibstomp stomp  [flags] file.bib...   rewrite cite keys (and optionally fields)
  bibstomp keys   [flags] file.bib...   report old -> new keys, no writes
                 -show changed|unchanged|all  (default changed)
                 -color auto|always|never     orange = changed, teal = kept
  bibstomp remap  -map m.tsv file.tex   rewrite \cite{} keys in source files
  bibstomp fold   [flags]               stdin -> stdout ASCII filter

run 'bibstomp <cmd> -h' for flags
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "stomp":
		err = cmdStomp(os.Args[2:], true)
	case "keys":
		err = cmdStomp(os.Args[2:], false)
	case "remap":
		err = cmdRemap(os.Args[2:])
	case "fold":
		err = cmdFold(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bibstomp:", err)
		os.Exit(1)
	}
}

func cmdStomp(args []string, write bool) error {
	fs := flag.NewFlagSet("stomp", flag.ExitOnError)
	o := bibstomp.DefaultKeyOpts()
	fs.StringVar(&o.Template, "template", o.Template,
		"key template: {author} {authors} {year} {shortyear} {title} {journal} {doi} {type}")
	fs.IntVar(&o.MaxAuthor, "authors", o.MaxAuthor, "max surnames in {authors}")
	fs.StringVar(&o.Joiner, "joiner", o.Joiner, "separator between surnames")
	fs.BoolVar(&o.IncludeVon, "von", false, "fold particles (van, de, von) into the surname")
	fs.StringVar(&o.Case, "case", o.Case, "preserve|lower|upper")
	fs.StringVar(&o.EtAl, "etal", "", "append to {authors} when truncated, e.g. EtAl")

	inPlace := fs.Bool("i", false, "rewrite files in place")
	backup := fs.String("backup", ".bak", "in-place backup suffix (empty to disable)")
	mapOut := fs.String("map", "", "write old<TAB>new key map to this file")
	fields := fs.String("fields", "", "comma-separated fields to normalise, or 'all'")
	normalize := fs.Bool("normalize", false, "shorthand for -fields on the usual human-readable fields")
	enc := fs.String("field-encoding", "utf8", "utf8|ascii -- how normalised fields are written")
	onlyDamaged := fs.Bool("only-damaged", false, "only remint keys that are non-ASCII or have separator runs")
	show := fs.String("show", "changed", "keys report: changed|unchanged|all")
	color := fs.String("color", "auto", "colourise the keys report: auto|always|never")
	verbose := fs.Bool("v", false, "print diagnostics to stderr")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch *show {
	case "changed", "unchanged", "all":
	default:
		return fmt.Errorf("-show must be changed, unchanged or all")
	}
	if err := setColorMode(*color); err != nil {
		return err
	}

	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	usesStdin := false
	for _, p := range paths {
		if p == "-" {
			usesStdin = true
		}
	}
	if usesStdin && *inPlace {
		return fmt.Errorf("-i cannot rewrite stdin in place")
	}

	normFields := map[string]bool{}
	allFields := *fields == "all"
	for _, f := range strings.Split(*fields, ",") {
		if f = strings.TrimSpace(strings.ToLower(f)); f != "" {
			normFields[f] = true
		}
	}
	if *normalize {
		for _, f := range []string{
			"author", "editor", "title", "booktitle",
			"journal", "publisher", "series", "institution", "school", "note",
		} {
			normFields[f] = true
		}
	}

	// Read every file, mint a key for every entry. Files are held rather than
	// streamed only so the splices can be applied per file at the end.
	type item struct {
		file  int
		entry *bibstomp.Entry
		key   string
	}
	var files []struct {
		path string
		src  []byte
	}
	var items []*item
	var allDiags []bibstomp.Diagnostic

	for _, path := range paths {
		src, err := readSource(path)
		if err != nil {
			return err
		}
		entries, warns := bibstomp.ParseBib(src)
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "%s: parse: %s\n", path, w)
		}
		fi := len(files)
		files = append(files, struct {
			path string
			src  []byte
		}{path, src})
		for _, e := range entries {
			if e.Special {
				continue
			}
			k, diags := bibstomp.MintKey(e, o)
			allDiags = append(allDiags, diags...)
			if *onlyDamaged && bibstomp.KeyIsClean(e.Key) {
				k = e.Key
			}
			items = append(items, &item{fi, e, k})
		}
	}

	// Report -- but never silently repair -- keys claimed twice. Renaming one
	// of them is not this tool's job: a single-entry stream cannot see the
	// library it is being appended to, so any suffix minted here would be
	// invented from an incomplete picture and would look authoritative anyway.
	firstUse := map[string]*bibstomp.Entry{}
	for _, it := range items {
		if prev, dup := firstUse[it.key]; dup {
			allDiags = append(allDiags, bibstomp.Diagnostic{
				it.entry.Key, "collision",
				"key " + it.key + " is also minted for " + prev.Key,
			})
			continue
		}
		firstUse[it.key] = it.entry
	}

	// Pass 3: splice. report carries every entry so the keys command can show
	// unchanged keys too; mapping carries only the renames, because that is
	// what remap consumes and an identity pair there is just noise.
	var mapping, report [][2]string
	splicesByFile := make([][]bibstomp.Splice, len(files))
	for _, it := range items {
		e := it.entry
		report = append(report, [2]string{e.Key, it.key})
		if it.key != e.Key {
			mapping = append(mapping, [2]string{e.Key, it.key})
			splicesByFile[it.file] = append(splicesByFile[it.file],
				bibstomp.Splice{e.KeyStart, e.KeyEnd, it.key})
		}
		if allFields || len(normFields) > 0 {
			for _, f := range e.Fields {
				if !allFields && !normFields[f.Name] {
					continue
				}
				nv := normaliseField(f, *enc)
				if nv != f.Value {
					splicesByFile[it.file] = append(splicesByFile[it.file],
						bibstomp.Splice{f.ValueStart, f.ValueEnd, nv})
				}
			}
		}
	}

	for i, fl := range files {
		out := bibstomp.ApplySplices(fl.src, splicesByFile[i])
		switch {
		case !write:
			// keys mode: report only
		case *inPlace:
			if *backup != "" {
				if err := os.WriteFile(fl.path+*backup, fl.src, 0o644); err != nil {
					return err
				}
			}
			if err := os.WriteFile(fl.path, out, 0o644); err != nil {
				return err
			}
		default:
			os.Stdout.Write(out)
		}
	}

	if !write {
		w := bufio.NewWriter(os.Stdout)
		for _, m := range report {
			same := m[0] == m[1]
			if (*show == "changed" && same) || (*show == "unchanged" && !same) {
				continue
			}
			fmt.Fprintln(w, renderRow(m[0], m[1], !same))
		}
		w.Flush()
	}
	if *mapOut != "" {
		var b strings.Builder
		for _, m := range mapping {
			fmt.Fprintf(&b, "%s\t%s\n", m[0], m[1])
		}
		if err := os.WriteFile(*mapOut, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	if *verbose {
		printDiags(allDiags)
	} else {
		n := 0
		for _, d := range allDiags {
			if d.Kind == "residue" || d.Kind == "macro" {
				n++
			}
		}
		if n > 0 {
			fmt.Fprintf(os.Stderr, "bibstomp: %d entries need review (-v to list)\n", n)
		}
	}
	return nil
}

func printDiags(ds []bibstomp.Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool { return ds[i].Kind < ds[j].Kind })
	for _, d := range ds {
		fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", d.Kind, d.Key, d.Detail)
	}
}

func normaliseField(f bibstomp.Field, enc string) string {
	detexed, _ := bibstomp.Detex(f.Value)
	if enc == "ascii" {
		folded, _ := bibstomp.Fold(detexed)
		return folded
	}
	return detexed
}

var errBadColor = fmt.Errorf("-color must be auto, always or never")

var (
	citeRe       = regexp.MustCompile(`\\[a-zA-Z]*cite[a-zA-Z]*\s*(\[[^\]]*\]\s*)*\{([^}]*)\}`)
	pandocCiteRe = regexp.MustCompile(`@([A-Za-z0-9_:.\-]+)`)
)

func cmdRemap(args []string) error {
	fs := flag.NewFlagSet("remap", flag.ExitOnError)
	mapFile := fs.String("map", "", "old<TAB>new key map produced by 'bibstomp keys'")
	inPlace := fs.Bool("i", false, "rewrite files in place")
	pandoc := fs.Bool("pandoc", false, "also rewrite pandoc-style @key citations")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *mapFile == "" {
		return fmt.Errorf("-map is required")
	}
	raw, err := os.ReadFile(*mapFile)
	if err != nil {
		return err
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 2)
		if len(parts) == 2 && parts[0] != "" {
			m[parts[0]] = parts[1]
		}
	}
	for _, path := range fs.Args() {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := citeRe.ReplaceAllFunc(src, func(b []byte) []byte {
			sub := citeRe.FindSubmatch(b)
			inner := string(sub[2])
			keys := strings.Split(inner, ",")
			for i, k := range keys {
				t := strings.TrimSpace(k)
				if nk, ok := m[t]; ok {
					keys[i] = nk
				}
			}
			return []byte(strings.Replace(string(b), inner, strings.Join(keys, ","), 1))
		})
		if *pandoc {
			out = pandocCiteRe.ReplaceAllFunc(out, func(b []byte) []byte {
				if nk, ok := m[string(b[1:])]; ok {
					return []byte("@" + nk)
				}
				return b
			})
		}
		if *inPlace {
			if err := os.WriteFile(path, out, 0o644); err != nil {
				return err
			}
		} else {
			os.Stdout.Write(out)
		}
	}
	return nil
}

func cmdFold(args []string) error {
	fs := flag.NewFlagSet("fold", flag.ExitOnError)
	detex := fs.Bool("detex", true, "decode TeX macros before folding")
	strict := fs.Bool("strict", false, "exit non-zero if any rune could not be mapped")
	if err := fs.Parse(args); err != nil {
		return err
	}
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	s := string(in)
	if *detex {
		s, _ = bibstomp.Detex(s)
	}
	folded, residue := bibstomp.Fold(s)
	os.Stdout.WriteString(folded)
	if len(residue) > 0 {
		fmt.Fprintf(os.Stderr, "bibstomp: unmapped: %s\n", string(residue))
		if *strict {
			os.Exit(1)
		}
	}
	return nil
}

// readSource reads a named file, or stdin when path is "-". Reading stdin
// makes the tool composable with the DOI resolver directly:
//
//	curl -LH "Accept: application/x-bibtex" https://doi.org/10.1093/... \\
//	  | bibstomp stomp >> library.bib
func readSource(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

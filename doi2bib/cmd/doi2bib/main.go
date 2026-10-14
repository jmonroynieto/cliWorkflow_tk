package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jmonroynieto/cliWorkflow_tk/doi2bib"
)

func run(args []string, in io.Reader, out, diagnostics io.Writer) int {
	return runWith(args, in, out, diagnostics, (doi2bib.Client{OnWarning: func(s string) { fmt.Fprintln(diagnostics, s) }}).Lookup, copyClipboard)
}

func copyClipboard(result string) error {
	cmd := exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(result)
	return cmd.Run()
}

func runWith(args []string, in io.Reader, out, diagnostics io.Writer, lookup func(context.Context, string) (string, error), copyText func(string) error) int {
	flags := flag.NewFlagSet("doi2bib", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	file := flags.String("oneshot", "", "Read one DOI per line from file")
	bare := flags.Bool("bare", false, "Output BibTeX without Markdown fencing")
	noClipboard := flags.Bool("no-clipboard", false, "Do not copy results to the X11 clipboard")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(diagnostics, "Unexpected positional arguments")
		return 2
	}
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
		defer f.Close()
		in = f
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#D6BA7C"))
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	failed := false
	for {
		if *file == "" {
			fmt.Fprint(diagnostics, style.Render("Enter DOI (empty to quit): "))
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if *file == "" {
				break
			}
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		bib, err := lookup(ctx, line)
		cancel()
		if err != nil {
			fmt.Fprintln(diagnostics, style.Render(err.Error()))
			failed = true
			continue
		}
		result := bib + "\n"
		if !*bare {
			result = doi2bib.Fenced(bib)
		}
		fmt.Fprintln(out, result)
		if !*noClipboard {
			if err := copyText(result); err != nil {
				fmt.Fprintln(diagnostics, style.Render("Clipboard unavailable: "+err.Error()))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(diagnostics, err)
		failed = true
	}
	if failed {
		return 1
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

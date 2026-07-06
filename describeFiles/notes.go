package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pydpll/errorutils"
)

var (
	Version  string
	Revision = "0"
	CommitId string
)

const (
	notesFileName = "description.notes"
	programMSG    = "\t\033[33m"
	colorReset    = "\033[0m"
)

// record holds a note entry for a file.
type record struct {
	note string
	date string
	tag  string // NEW: optional tag for categorization, filtering, and listing
}

func (r record) String() string {
	if r.tag != "" {
		return fmt.Sprintf("%s -- %s [tag: %s]", r.date, r.note, r.tag)
	}
	return fmt.Sprintf("%s -- %s", r.date, r.note)
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		tellUSR("Looking for notes in this directory:")
		rows, err := retrieveNotesTable()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				tellUSR("no notes in current directory")
				os.Exit(1)
			}
			tellUSR("Error reading notes file:", err.Error())
			os.Exit(1)
		}
		descriptions := showTable(rows) // bubbletea model
		if _, err := tea.NewProgram(model{descriptions}).Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
		return

	}
	switch args[0] {
	case "-h", "--help":
		fmt.Println(helptext)
		return
	case "-v", "--version", "-version":
		fmt.Printf("describeFiles version %s.%s (%s)\n", Version, Revision, CommitId)
		return
	}

	notes, err := readNotesFile()
	if err != nil {
		tellUSR("Error reading notes file:", err.Error())
		os.Exit(1)
	}

	for _, filename := range args {
		noteRecord, ok := notes[filename]
		if ok {
			tellUSR("Previous note:", fmt.Sprint(noteRecord))
		}

		newNote := readNoteFromPrompt(filename)
		newTag := readTagFromPrompt(filename)
		now := time.Now()
		timestamp := now.Format("2006-Feb-02 15:04:05")
		notes[filename] = record{note: newNote, date: timestamp, tag: newTag}
	}

	err = writeNotesFile(notes)
	if err != nil {
		tellUSR("Error writing notes file:", err.Error())
		os.Exit(1)
	}

	tellUSR("Notes saved successfully")
}

func readNotesFile() (map[string]record, error) {
	notes := make(map[string]record)
	data, err := os.ReadFile(notesFileName)
	if os.IsNotExist(err) {
		return notes, nil
	}
	if err != nil {
		return notes, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) == 3 {
			notes[fields[1]] = record{
				date: fields[0],
				note: fields[2],
				tag:  "",
			}
		} else if len(fields) == 4 {
			notes[fields[1]] = record{
				date: fields[0],
				note: fields[2],
				tag:  fields[3],
			}
		}
		// malformed lines are silently ignored (consistent with original lenient behavior for non-3)
	}
	return notes, nil
}

func readNoteFromPrompt(filename string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Ente note for file %s:\n", filename)
	note, err := reader.ReadString('\n')
	if err == io.EOF {
		err = nil
	}
	errorutils.ExitOnFail(err, errorutils.WithMsg(fmt.Sprintf("Unexpected error while taking in new note %v", err)))

	return strings.TrimSpace(note)
}

func readTagFromPrompt(filename string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Tag for file %s (Enter to skip):\n", filename)
	tag, err := reader.ReadString('\n')
	if err == io.EOF {
		err = nil
	}
	errorutils.ExitOnFail(err, errorutils.WithMsg(fmt.Sprintf("Unexpected error while taking in tag for %s: %v", filename, err)))

	return strings.TrimSpace(tag)
}

func writeNotesFile(notes map[string]record) error {
	var b strings.Builder
	for filename, rec := range notes {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", rec.date, filename, rec.note, rec.tag)
	}
	return os.WriteFile(notesFileName, []byte(b.String()), 0o644)
}

func tellUSR(message ...string) {
	fmt.Println(programMSG, strings.Join(message, " "), colorReset)
}

func retrieveNotesTable() ([]table.Row, error) {
	data, err := os.ReadFile(notesFileName)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var rowmaker []table.Row
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		var row table.Row
		if len(fields) == 3 {
			// Backwards compatibility: old format without tag
			row = table.Row{fields[0], fields[1], fields[2], ""}
		} else if len(fields) == 4 {
			row = table.Row{fields[0], fields[1], fields[2], fields[3]}
		} else {
			errorutils.ExitOnFail(fmt.Errorf("error: table malformed — row with %d fields (expected 3 or 4): %q", len(fields), fields))
			continue
		}
		rowmaker = append(rowmaker, row)
	}
	errorutils.ExitOnFail(scanner.Err())
	return rowmaker, nil
}

var helptext = `
describeFiles — attach timestamped notes (and tags) to files in a directory

USAGE:
  describeFiles                  Launch the interactive TUI to browse existing notes
  describeFiles <file> [files...]  Add or update a note (and optional tag) for the given file(s)

OPTIONS:
  -h, --help     Show this help message
  -v, --version  Show version information

ADDING / UPDATING NOTES:
  • Run describeFiles with one or more filenames as arguments.
  • The filename acts as the unique key — last entry overwrites.
  • Tags make it easy to filter and group notes later (visible in TUI and grep-friendly output).

TUI KEYBOARD SHORTCUTS:
  ↑ / ↓            Move selection
  Enter            Print "filename:note #tag" (grep-friendly) and exit
  q / Ctrl+C       Quit

STORAGE:
  • Notes are stored in ./description.notes (tab-delimited, human-readable, version-control friendly)
  • Format: timestamp<TAB>filename<TAB>note<TAB>tag
  `

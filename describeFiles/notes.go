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
	Revision = ".0"
	CommitId string
)

const (
	notesFileName = "description.notes"
	programMSG    = "\t\033[33m"
	colorReset    = "\033[0m"
)

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
		fmt.Println("I'll tell you how to use it soon. Sorry #todo")
		return
	case "-v", "--version", "-version":
		fmt.Printf("describeFiles version %s%s (%s)\n", Version, Revision, CommitId)
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
		now := time.Now()
		timestamp := now.Format("2006-Feb-02 15:04:05")
		notes[filename] = record{note: newNote, date: timestamp}
	}

	err = writeNotesFile(notes)
	if err != nil {
		tellUSR("Error writing notes file:", err.Error())
		os.Exit(1)
	}

	tellUSR("Notes saved successfully")
}

type record struct {
	note string
	date string
}

func (r record) String() string {
	return fmt.Sprintf("%s -- %s", r.date, r.note)
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
		fields := strings.Split(line, "\t")
		if len(fields) == 3 {
			notes[fields[1]] = record{date: fields[0], note: fields[2]}
		}

	}
	return notes, nil
}

func readNoteFromPrompt(filename string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter note for file %s:\n", filename)
	note, err := reader.ReadString('\n')
	if err == io.EOF {
		err = nil
	}
	errorutils.ExitOnFail(err, errorutils.WithMsg(fmt.Sprintf("Unexpected error while taking in new note %v", err)))

	return strings.TrimSpace(note)
}

func writeNotesFile(notes map[string]record) error {
	var lines []string
	for filename, record := range notes {
		line := fmt.Sprintf("%s\t%s\t%s\n", record.date, filename, record.note)
		lines = append(lines, line)
	}

	data := []byte(strings.Join(lines, "\n") + "\n")
	err := os.WriteFile(notesFileName, data, 0o644)

	return err
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
		line := scanner.Text()
		fields := strings.Split(line, "\t") // expects three
		if len(fields) != 3 && (len(fields) > 2) {
			errorutils.ExitOnFail(fmt.Errorf("error: table malformed there is a row with malformed fields %q of length: %d", fields, len(fields)))
		}
		if fields[0] == "" {
			continue
		}
		rowmaker = append(rowmaker, table.Row(fields))
	}
	errorutils.ExitOnFail(scanner.Err())
	// old printer
	//	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, '\t', 0)
	return rowmaker, nil
}

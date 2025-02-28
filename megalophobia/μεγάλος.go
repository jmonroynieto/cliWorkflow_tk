package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
)

var (
	stdoutBuffer     []string
	bufferMutex      sync.Mutex
	origStdout       *os.File
	pipeReader       *os.File
	pipeWriter       *os.File
	stopChan         chan struct{}
	lineChan         chan string
	wg               sync.WaitGroup
	lastDisplayLines int
)

func AsyncUpdateBuffer() {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()
	for line := range lineChan {
		changed := updateBuffer(line)
		if changed {
			displayBuffer()
		}
	}
	pipeWriter.Close()
	TeardownCapture()
}

// SetupCapture redirects stdout to a pipe and starts the *[CONCURRENT]* capture goroutine.
func SetupCapture() error {
	lineChan = make(chan string, 30)
	var err error
	origStdout = os.Stdout
	pipeReader, pipeWriter, err = os.Pipe()
	if err != nil {
		return err
	}
	os.Stdout = pipeWriter
	stopChan = make(chan struct{}, 2)
	wg.Add(1)
	go captureOutput(pipeReader)
	return nil
}

func captureOutput(pr *os.File) {
	defer wg.Done()
	scanner := bufio.NewScanner(pr)
chanWatcher:
	for {
		select {
		case _, more := <-stopChan:
			close(lineChan)
			if !more {
				break chanWatcher
			}
		default:
			if scanner.Scan() {
				inspectLine := scanner.Text()
				lineChan <- inspectLine
			} else {
				continue
			}
		}
	}
	if scanner.Err() != nil {
		logrus.Error(scanner.Err())
	}
}

// updateBuffer appends a new line to the buffer — keeping only the last three lines.
func updateBuffer(line string) (changed bool) {

	if line == "" {
		return false
	}
	for _, existingLine := range stdoutBuffer {
		// skip the line's timestamp when updating the buffer start at [...
		//2025-02-27 17:35:55 [INFO] Running jobs — [02/7157a4 16/3a6f0a a1/83bb29 c6/7a55a2]
		rg := regexp.MustCompile(`^.{20}\[.*([A-Z]{4})\.*].*`)
		if isLog := rg.MatchString(line); isLog && existingLine[20:] == line[20:] {
			return false
		} else if existingLine == line {
			return false
		}
	}

	stdoutBuffer = append(stdoutBuffer, line)
	if len(stdoutBuffer) > 3 {
		stdoutBuffer = stdoutBuffer[len(stdoutBuffer)-3:]
	}
	return true
}

func truncateLine(line string, maxWidth int) string {
	if len(line) > maxWidth-3 {
		return line[:maxWidth-3] + "…"
	}
	return line
}

// displayBuffer overwrites the previous display using ANSI escape codes.
func displayBuffer() {
	header := "\033[1;32m--- Program Output Display ---\033[0m"
	footer := "\033[1;32m--- End Display --------------\033[0m"
	clearLine := "\033[2K" // Clear the entire line

	maxWidth, _, err := terminal.GetSize(int(origStdout.Fd()))
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.Debugf("Terminal width: %d", maxWidth)
		lastDisplayLines += 1
	}
	errorutils.WarnOnFail(err)

	if lastDisplayLines > 0 {
		fmt.Fprintf(origStdout, "\033[%dA", lastDisplayLines) // Move cursor up
		for i := 0; i < lastDisplayLines; i++ {
			fmt.Fprint(origStdout, clearLine+"\033[1B") // Clear and move down
		}
		fmt.Fprintf(origStdout, "\033[%dA", lastDisplayLines) // Reset cursor
	}

	lines := []string{header}
	lines = append(lines, stdoutBuffer...)
	lines = append(lines, footer)

	for _, line := range lines {
		fmt.Fprintf(origStdout, "%-80s\n", truncateLine(line, maxWidth)) // Ensure fixed width for stability
	}
	lastDisplayLines = len(lines)
}

// TeardownCapture stops capturing and restores the original stdout.
func TeardownCapture() error {
	wg.Wait()
	os.Stdout = origStdout
	clearDisplayWindow()
	return nil
}

func clearDisplayWindow() {
	if lastDisplayLines > 0 {
		// Move up by lastDisplayLines
		fmt.Fprintf(origStdout, "\033[%dA", lastDisplayLines)
		// Clear each line and move down
		for i := 0; i < lastDisplayLines; i++ {
			fmt.Fprint(origStdout, "\033[2K\033[1B")
		}
		// Move back to the starting position
		fmt.Fprintf(origStdout, "\033[%dA", lastDisplayLines)
	}
}

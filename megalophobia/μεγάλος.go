package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	stdoutBuffer     []string
	bufferMutex      sync.Mutex
	origStdout       *os.File
	pipeReader       *os.File
	pipeWriter       *os.File
	stopChan         chan struct{}
	wg               sync.WaitGroup
	lastDisplayLines int
)

// SetupCapture redirects stdout to a pipe and starts the capture goroutine.
func SetupCapture() error {
	var err error
	origStdout = os.Stdout
	pipeReader, pipeWriter, err = os.Pipe()
	if err != nil {
		return err
	}
	os.Stdout = pipeWriter
	stopChan = make(chan struct{})
	wg.Add(1)
	go captureOutput()
	return nil
}

// captureOutput reads from the pipe and updates the display window.
func captureOutput() {
	defer wg.Done()
	scanner := bufio.NewScanner(pipeReader)
	for {
		select {
		case <-stopChan:
			return
		default:
			if scanner.Scan() {
				updateBuffer(scanner.Text())
				displayBuffer()
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

// updateBuffer appends a new line to the buffer — keeping only the last three lines.
func updateBuffer(line string) {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()
	stdoutBuffer = append(stdoutBuffer, line)
	if len(stdoutBuffer) > 3 {
		stdoutBuffer = stdoutBuffer[len(stdoutBuffer)-3:]
	}
}

// displayBuffer overwrites the previous display using ANSI escape codes.
func displayBuffer() {
	header := "\033[1;32m--- Program Output Display ---\033[0m"
	footer := "\033[1;32m--- End Display --------------\033[0m"
	clearLine := "\033[2K" // Clear the entire line

	bufferMutex.Lock()
	defer bufferMutex.Unlock()

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
		fmt.Fprintf(origStdout, "%-80s\n", line) // Ensure fixed width for stability
	}
	lastDisplayLines = len(lines)
}

// func displayBuffer() {
// 	bufferMutex.Lock()
// 	defer bufferMutex.Unlock()
// 	if lastDisplayLines > 0 {
// 		// Move cursor up by lastDisplayLines, clear each line, and return to start.
// 		fmt.Fprintf(origStdout, "\033[%dA", lastDisplayLines)
// 		for i := 0; i < lastDisplayLines; i++ {
// 			fmt.Fprint(origStdout, "\033[2K\033[1B")
// 		}
// 		fmt.Fprintf(origStdout, "\033[%dA", lastDisplayLines)
// 	}
// 	// Build and print the new display block.
// 	lines := []string{header}
// 	lines = append(lines, stdoutBuffer...)
// 	lines = append(lines, footer)
// 	for _, line := range lines {
// 		fmt.Fprintln(origStdout, line)
// 	}
// 	lastDisplayLines = len(lines)
// }

// TeardownCapture stops capturing and restores the original stdout.
func TeardownCapture() error {
	close(stopChan)
	pipeWriter.Close()
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

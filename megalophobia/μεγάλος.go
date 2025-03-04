package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
)

var (
	//output
	origStdout       *os.File
	stdoutBuffer     []string
	lastDisplayLines int
	pipeReader       *os.File
	pipeWriter       *os.File
	//concurrenty
	lineChan          chan string
	terminationSignal chan struct{}
	wg                sync.WaitGroup
	bufferMutex       sync.Mutex
	ὄλεθροςπάντων     = "Ὦ χάϝος, ὁ μέγας ἄβυσσος σε κατέφαγεν!" // end of input, terminator
)

// redirects stdout to a pipe and starts the *[CONCURRENT]* routines that capture and display the output
func SetupCapture() (*bool, error) {
	var running bool = true
	var err error
	{ //handle interrupt
		interrupted := make(chan os.Signal, 1)
		signal.Notify(interrupted, syscall.SIGINT)
		go func() {
			//logrus.Debug("Starting interrupt watcher loop")
			for sig := range interrupted {
				//logrus.Debug("Interrupt signal received")
				if sig == syscall.SIGINT {
					fmt.Fprintf(origStdout, "\033[2K\033[1A")
					running = false
					fmt.Println(ὄλεθροςπάντων)
					break
				}
			}
			//logrus.Debug("Finished interrupt watcher loop")
		}()
	}
	lineChan = make(chan string, 30)
	terminationSignal = make(chan struct{}, 2)
	origStdout = os.Stdout
	//set to devnull during debugging
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		origStdout, _ = os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	}
	pipeReader, pipeWriter, err = os.Pipe()
	if err != nil {
		return &running, err
	}
	os.Stdout = pipeWriter //output swap
	//workers
	wg.Add(1)
	go captureOutput()
	go asyncUpdateBuffer(lineChan)
	return &running, nil
}

// sends termination signal and blocks main goroutine
func FinishCapture() {
	//fmt.Println(ὄλεθροςπάντων) //last ditch effort to close the output
	pipeWriter.Close()
	logrus.Debug("Termination signal sent.")
	close(terminationSignal)
	wg.Wait() //necessary to run: blocks main goroutine
}

//
//main program workers: control flow
//

func captureOutput() {
	scanner := bufio.NewScanner(pipeReader)
	var τέλος bool
	logrus.Debug("CAPTURE: Starting channel watcher loop")
chanWatcher:
	for {
		logrus.Debug("CAPTURE: Starting capture loop iteration")
		select {
		case _, more := <-terminationSignal: // termination sequence
			if !more {
				close(lineChan)
				pipeReader.Close()
				logrus.Debug("CAPTURE: No more termination signals, breaking loop")
				break chanWatcher
			}
		default:
			logrus.Debug("CAPTURE: Received no termination signal")
			if τέλος {
				logrus.Debug("CAPTURE: Found τέλος previously, no other line will be processed")
				continue
			}
			if scanner.Scan() {
				logrus.Debug("CAPTURE: Scanner returned true")
				// check for the end
				if scanner.Text() == ὄλεθροςπάντων {
					logrus.Debug("CAPTURE: Found the end of the output")
					τέλος = true
					continue
				}
				logrus.Debug("CAPTURE: the end is not here yet")
				inspectLine := scanner.Text()
				logrus.Debug("CAPTURE: Scanned line: " + inspectLine)
				lineChan <- inspectLine
				logrus.Debug("CAPTURE: Line sent to channel")
				continue
			}
			logrus.Debug("CAPTURE: Scanner returned false, breaking loop")
		}
	}
	logrus.Debug("CAPTURE: Finished capture watcher loop")
	if scanner.Err() != nil {
		logrus.Error(scanner.Err())
	}
}

func asyncUpdateBuffer(feedline chan string) {
	defer func() {
		logrus.Debug("ASYUPBF: deferred wg.Done() called")
		wg.Done()
	}()
	bufferMutex.Lock()
	defer func() {
		logrus.Debug("ASYUPBF: deferred writerMutex.Unlock() called")
		bufferMutex.Unlock()
	}()
	for line := range feedline {
		logrus.Debug("ASYUPBF: about to call updateBuffer")
		changed := updateBuffer(line)
		logrus.Debug("ASYUPBF: about to check if updateBuffer returned true")
		if changed {
			logrus.Debug("ASYUPBF: about to call displayBuffer")
			displayBuffer()
		}
		logrus.Debug("ASYUPBF: about to finish a loop iteration through feedline")
	}
	logrus.Debug("ASYUPBF: about to call teardownCapture")
	teardownCapture()
}

//
//Subsidiary functions: operation logic
//

func teardownCapture() error {
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

func updateBuffer(line string) (changed bool) {

	if line == "" {
		return false
	}
	// compare by skipping the line's timestamp for log messages formated by pydpll/errorutils formatter
	for _, existingLine := range stdoutBuffer {
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

// overwrites the previous display using ANSI escape codes.
func displayBuffer() {
	header := "\033[1;32m--- Program Output Display ---\033[0m"
	footer := "\033[1;32m--- End Display --------------\033[0m"
	clearLine := "\033[2K" //Clear the entire line

	maxWidth, _, err := terminal.GetSize(int(origStdout.Fd()))
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		maxWidth = 80
		logrus.Tracef("Terminal width: %d", maxWidth)
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

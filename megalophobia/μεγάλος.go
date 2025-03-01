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
	go captureOutput(pipeReader)
	go asyncUpdateBuffer(lineChan, pipeWriter)
	return &running, nil
}

// sends termination signal and blocks main goroutine
func FinishCapture() {
	fmt.Println(ὄλεθροςπάντων)
	logrus.Debug("Termination signal sent.")
	close(terminationSignal)
	wg.Wait() //necessary to run: blocks main goroutine
}

//
//main program workers: control flow
//

func captureOutput(maskFunnel *os.File) {
	scanner := bufio.NewScanner(maskFunnel)
	var τέλος bool
	logrus.Debug("Starting channel watcher loop")
chanWatcher:
	for {
		logrus.Debug("Starting  capture loop iteration")
		select {
		case _, more := <-terminationSignal: // termination sequence
			if !more {
				close(lineChan)
				logrus.Debug("No more termination signals, breaking loop")
				break chanWatcher
			}
		default:
			logrus.Debug("Received no termination signal")
			if τέλος {
				logrus.Debug("Found τέλος previoulsy, no other line will be processed")
				continue
			}
			if scanner.Scan() {
				logrus.Debug("Scanner returned true")
				//check for the end
				if scanner.Text() == ὄλεθροςπάντων {
					logrus.Debug("Found the end of the output")
					τέλος = true
					continue
				}
				logrus.Debug("the end is not here yet")
				inspectLine := scanner.Text()
				logrus.Debug("Scanned line: " + inspectLine)
				lineChan <- inspectLine
				logrus.Debug("Line sent to channel")
				continue
			}
			logrus.Debug("Scanner returned false, breaking loop")
			break chanWatcher
		}
	}
	logrus.Debug("Finished capture watcher loop")
	if scanner.Err() != nil {
		logrus.Error(scanner.Err())
	}
}

func asyncUpdateBuffer(feedline chan string, outputMask *os.File) {
	defer func() {
		logrus.Debug("deferred wg.Done() called in asyncUpdateBuffer")
		wg.Done()
	}()
	bufferMutex.Lock()
	defer func() {
		logrus.Debug("deferred writerMutex.Unlock() called in asyncUpdateBuffer")
		bufferMutex.Unlock()
	}()
	for line := range feedline {
		//logrus.Debug("about to call updateBuffer in asyncUpdateBuffer")
		changed := updateBuffer(line)
		//logrus.Debug("about to check if updateBuffer returned true in asyncUpdateBuffer")
		if changed {
			//logrus.Debug("about to call displayBuffer in asyncUpdateBuffer")
			displayBuffer()
		}
		//logrus.Debug("about to finish a loop through feedline in asyncUpdateBuffer")
	}
	//logrus.Debug("about to close outputMask in asyncUpdateBuffer")
	outputMask.Close()
	//logrus.Debug("about to call teardownCapture in asyncUpdateBuffer")
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

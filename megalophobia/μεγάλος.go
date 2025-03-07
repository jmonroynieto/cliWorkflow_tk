package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
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
	terminationSignal chan struct{}
	wg                sync.WaitGroup
	bufferMutex       sync.Mutex
	ὄλεθροςπαντός     = "ὦ χάϝος, ἡ μεγάλη ἄβυσσος σε κατέφαγεν!" // debug end of input, terminator
)

// redirects stdout to a pipe and starts the *[CONCURRENT]* routines that capture and display the output
func SetupCapture() (*bool, error) {
	running := true
	var err error
	origStdout = os.Stdout
	//set to devnull during debugging
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		origStdout, _ = os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	}

	{ //handle interrupt
		interrupted := make(chan os.Signal, 1)
		signal.Notify(interrupted, syscall.SIGINT)
		go func() {
			for sig := range interrupted {
				if sig == syscall.SIGINT {
					running = false
					fmt.Fprintf(origStdout, "\033[2K\033[G")
					fmt.Println(ὄλεθροςπαντός)
					break
				}
			}
		}()
	}

	lineChan := make(chan string, 30)
	terminationSignal = make(chan struct{}, 2)
	pipeReader, pipeWriter, err = os.Pipe()
	if err != nil {
		return &running, err
	}
	os.Stdout = pipeWriter //output swap

	//workers
	wg.Add(1)
	go captureOutput(lineChan)
	go asyncUpdateBuffer(lineChan)
	return &running, nil
}

func FinishCapture() {
	//fmt.Println(ὄλεθροςπαντός) //last ditch effort to close the output
	pipeWriter.Close()
	close(terminationSignal)
	wg.Wait() //blocks main goroutine
}

//
//main program workers: control flow
//

func captureOutput(funnel chan string) {
	scanner := bufio.NewScanner(pipeReader)
	var τέλος bool
chanWatcher:
	for {
		select {
		case _, more := <-terminationSignal:
			if !more {
				close(funnel)
				break chanWatcher
			}
		default:
			if τέλος {
				continue
			}
			if scanner.Scan() {
				// check for the end
				if scanner.Text() == ὄλεθροςπαντός {
					τέλος = true
					continue
				}
				inspectLine := scanner.Text()
				funnel <- inspectLine
				continue
			}
		}
	}
	if scanner.Err() != nil {
		logrus.Error(scanner.Err())
	}
}

func asyncUpdateBuffer(feedline chan string) {
	defer func() {
		wg.Done()
	}()
	bufferMutex.Lock()
	defer func() {
		bufferMutex.Unlock()
	}()
	for line := range feedline {
		changed := updateBuffer(line)
		if changed {
			displayBuffer()
		}
	}
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
		fmt.Fprintf(origStdout, "\033[%dA\033[G", lastDisplayLines)
	}
}

func updateBuffer(line string) (changed bool) {

	if line == "" {
		return false
	}
	for _, existingLine := range stdoutBuffer {
		if len(line) < 31 || line[26:30] != "INFO" {
			break
		}
		if existingLine[26:30] == "INFO" && existingLine[20:] == line[20:] {
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

func displayBuffer() {
	header := "\033[1;32m--- " + ὄλεθροςπαντός + " ---\033[0m"
	footer := "\033[1;32m--------------- ὄλεθρος παντός ---------------\033[0m"
	clearLine := "\033[2K" //Clear the entire line

	maxWidth, _, err := terminal.GetSize(int(origStdout.Fd()))
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		maxWidth = 80
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
		fmt.Fprintf(origStdout, "%-80s\n", truncateLine(line, maxWidth)) // Ensure fixed width
	}
	lastDisplayLines = len(lines)
}

func truncateLine(line string, maxWidth int) string {
	if len(line) > maxWidth-3 {
		return line[:maxWidth-3] + "…"
	}
	return line
}

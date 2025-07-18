package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"time"
)

var (
	Version  string
	Revision = ".0"
	CommitId string
	color    string
	old      string
)

var colors = []string{
	"\033[31m",       // Red
	"\033[32m",       // Green
	"\033[33m",       // Yellow
	"\033[34m",       // Blue
	"\033[35m",       // Magenta
	"\033[36m",       // Cyan
	"\033[37m",       // White
	"\033[91m",       // Bold red
	"\033[92m",       // Bold green
	"\033[93m",       // Bold yellow
	"\033[94m",       // Bold blue
	"\033[95m",       // Bold magenta
	"\033[96m",       // Bold cyan
	"\033[97m",       // Bold white
	"\033[38;5;196m", // Light orange
	"\033[38;5;84m",  // Turquoise
	"\033[38;5;130m", // Light violet
	"\033[38;5;34m",  // Blueviolet
	"\033[38;5;208m", // Sea green
	"\033[38;5;166m", // Coral
	"\033[38;5;217m", // Light pink"
	"\033[38;5;183m", // lavender
}

var usage string = `Usage: dripC [-h | --help] [(-|--)EACHLN | (-|--)TIMED |]
	Modes can be styled as flags or arguments:
	TIMED: change color after a one-second delay in the input stream.
	EACHLN: change color for each line in the input stream.
skipping the mode argument would color all input a single color.`

func main() {
	if len(os.Args) > 3 || (len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help")) {
		fmt.Printf("dripC v%s%s (%s)\n", Version, Revision, CommitId)
		fmt.Println(usage)
		return
	} else if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Printf("dripC version %s%s (%s)\n", Version, Revision, CommitId)
		return
	}
	changeColor()
	// logrus.Debug("os.Args:", os.Args)
	if len(os.Args) == 2 {
		// logrus.Debug("mode specified")
		switch prep(os.Args[1]) {
		case "EACHLN":
			colorEach()
		case "TIMED":
			timedChange()
		default:
			fmt.Print("\033[0m")
			fmt.Println(usage)
			return
		}
	} else {
		// logrus.Debug("no mode specified, coloringblock")
		colorAll()
	}
	// reset terminal color
	fmt.Print("\033[0m")
}

// allows for the keyword to be upper or lowercase and used as an argument or flag
func prep(opt string) string {
	opt = strings.ToUpper(opt)
	opt = strings.ReplaceAll(opt, "-", "")
	return opt
}

func changeColor() {
	old = color
	c := colors[rand.IntN(len(colors))]
	if c != old {
		color = c
		fmt.Printf("%s", color)
		return
	}
	changeColor()
}

func colorAll() {
	// escape sequence consumes first character of input, provided zero-width space. if this line is deleted, the first character of input will be consumed.
	inputReader := bufio.NewReader(os.Stdin)
	outputWriter := bufio.NewWriter(os.Stdout)
	io.Copy(outputWriter, inputReader)
}

func colorEach() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// print scanner text to stdout with color
		fmt.Printf("%s\n", scanner.Text())
		changeColor()
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

func timedChange() {
	// fmt.Println("starting timed change")
	timedelay := 1500 * time.Millisecond
	input_ch := make(chan string, 5)
	timer := time.NewTimer(timedelay)
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			input_ch <- scanner.Text()
			// logrus.Debug("input received")
		}
		close(input_ch)
	}()
looper:
	for {
		select {
		case <-timer.C:
			// logrus.Debug("timer ended, changing color")
			changeColor()
		case input, ok := <-input_ch:
			// logrus.Debug("input received, timer reset")
			if !ok {
				break looper
			}
			fmt.Printf("%s\n", input)
			timer.Reset(timedelay)
		}
	}
	// logrus.Debug("exiting looper")
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input failed:", err)
	}
}

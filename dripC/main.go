package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	Version  string
	Revision = ".1"
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

var usage string = `Usage: dripC [-h | --help] [(-|--)EACHLN | (-|--)TIMED | (-|--)CARDS]
	all modes are meant for streaming input. and will display as it comes in.
	Modes can be styled as flags or arguments:
	TIMED: change color after a one-second delay in the input stream.
	EACHLN: change color for each line in the input stream.
	CARDS: print colored buttons per line in the input stream, intended for small items.
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
			colorEachLine()
		case "TIMED":
			timedChange()
		case "CARDS":
			cardPrint()
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

func colorEachLine() {
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

// use lipgloss to make cards
func cardPrint() {
	//reset colorchange
	fmt.Print("\033[0m")

	//take in items from stdin
	var linesIN_ch chan string = make(chan string, 5)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go renderCards(linesIN_ch, &wg)
	inputReader := bufio.NewScanner(os.Stdin)
	for inputReader.Scan() {
		linesIN_ch <- inputReader.Text()
	}
	close(linesIN_ch)
	if err := inputReader.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input failed:", err)
		os.Exit(1)
	}
	wg.Wait()
}

func renderCards(linesIN_ch chan string, wg *sync.WaitGroup) {
	if wg != nil {
		defer func() {
			wg.Done()
		}()
	}
	config := DefaultButtonConfig()
	var buffer = NewRollingBuffer(500)
	done_ch := make(chan bool)
	go func() {
		for msg := range linesIN_ch {
			if msg != "" {
				buffer.Add(msg)
			}
		}
		done_ch <- true
	}()

	//wraplines as a test that give us how many items to remove from the buffer
	//use a ticker to print when lines are full
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			lineItems := buffer.GetAll()
			wrappedLines := WrapItemsToLinesAuto(lineItems, config)
			//for the amout of items in wrapped lines, remove them from the buffer
			count := 0
			for _, line := range wrappedLines {
				for range line {
					count++
				}
			}
			buffer.RemoveFirst(count)
			for _, line := range wrappedLines {
				fmt.Println(lipglossMagic(line, config))
				fmt.Printf("\n")
			}
		case <-done_ch:
			ticker.Stop()
			//print remaining items in buffer
			lineItems := buffer.GetAll()
			wrappedLines := WrapItemsToLinesAuto(lineItems, config)
			for _, line := range wrappedLines {
				fmt.Println(lipglossMagic(line, config))
				fmt.Printf("\n")
			}
			return
		}
	}
}

func lipglossMagic(lineItems []string, config ButtonConfig) string {
	buttons := make([]string, len(lineItems))
	buttonStyle := lipgloss.NewStyle().Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0")).
		Padding(0, config.HorizontalPadding/2)

	for i, item := range lineItems {
		c := colors[rand.IntN(len(colors))]
		buttonStyle := buttonStyle.Background(ansiToLipgloss[c])
		buttons[i] = buttonStyle.Render(item)
	}
	sep := lipgloss.NewStyle().
		Margin(0, config.ItemMargin).
		Render(" ")

	return lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(buttons, sep))
}

var ansiToLipgloss = map[string]lipgloss.AdaptiveColor{
	"\033[31m":       {Light: "#ff5733", Dark: "#d81e05"},   // Burnt Orange
	"\033[32m":       {Light: "#50c878", Dark: "#00913a"},   // Emerald
	"\033[33m":       {Light: "#f4d03f", Dark: "#f7b204"},   // Mustard Yellow
	"\033[34m":       {Light: "#4a90e2", Dark: "#0b58a2"},   // Cerulean
	"\033[35m":       {Light: "#e066ff", Dark: "#4adf90ff"}, // Orchid
	"\033[36m":       {Light: "#69d2e7", Dark: "#00b5ad"},   // Aqua
	"\033[37m":       {Light: "#a6a6a6", Dark: "#e0e0e0"},   // Silver
	"\033[91m":       {Light: "#d83838", Dark: "#ff6b6b"},   // Crimson
	"\033[92m":       {Light: "#6aa84f", Dark: "#a7d28d"},   // Sage Green
	"\033[93m":       {Light: "#f1c40f", Dark: "#f7e75e"},   // Goldenrod
	"\033[94m":       {Light: "#2e86c1", Dark: "#5dade2"},   // Steel Blue
	"\033[95m":       {Light: "#8e44ad", Dark: "#b278d6"},   // Royal Purple
	"\033[96m":       {Light: "#48c9b0", Dark: "#82e0d1"},   // Mint
	"\033[97m":       {Light: "#d9d9d9", Dark: "#f2f2f2"},   // Platinum
	"\033[38;5;196m": {Light: "#e9724d", Dark: "#f09b7c"},   // Terracotta
	"\033[38;5;84m":  {Light: "#48c9b0", Dark: "#82e0d1"},   // Seafoam Green
	"\033[38;5;130m": {Light: "#c094f6", Dark: "#297f34ff"},   // Mauve
	"\033[38;5;34m":  {Light: "#673ab7", Dark: "#83081eff"}, // Amethyst
	"\033[38;5;208m": {Light: "#33725b", Dark: "#5c9a7e"},   // Forest Green
	"\033[38;5;166m": {Light: "#ff6f61", Dark: "#ff978a"},   // Coral Pink
	"\033[38;5;217m": {Light: "#f4a4c2", Dark: "#f7b2c9"},   // Dusty Rose
	"\033[38;5;183m": {Light: "#b9a6d4", Dark: "#d4c8e7"},   // Periwinkle
}

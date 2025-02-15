package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strings"
)

var color string
var old string

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
}

func main() {
	changeColor()
	if len(os.Args) == 2 && prep(os.Args[1]) == "EACHLN" {
		colorEach()
	} else {
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
		//print scanner text to stdout with color
		fmt.Printf("%s\n", scanner.Text())
		changeColor()
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"
)

var colors = []string{
	"\033[31m", // Red
	"\033[32m", // Green
	"\033[33m", // Yellow
	"\033[34m", // Blue
	"\033[35m", // Magenta
	"\033[36m", // Cyan
	"\033[37m", // White
	"\033[91m", // Bold red
	"\033[92m", // Bold green
	"\033[93m", // Bold yellow
	"\033[94m", // Bold blue
	"\033[95m", // Bold magenta
	"\033[96m", // Bold cyan
	"\033[97m", // Bold white
}

func main() {
	changeColor()
    opt := strings.ToUpper(os.Args[1])
    opt = strings.Replace(opt, "-", "",-1)
	if len(os.Args) == 2 && opt == "EACHLN" {
		colorEach()
	} else {
		colorAll()
	}
	// reset terminal color
	fmt.Print("\033[0m")
}

func changeColor() {
	rand.Seed(time.Now().UnixNano())
	color := colors[rand.Intn(len(colors))]
	fmt.Printf("%s\033", color)
	fmt.Printf("​")
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
		color := colors[rand.Intn(len(colors))]
		fmt.Printf("%s%s\033[0m\n", color, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

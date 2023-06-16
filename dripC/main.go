package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
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
	rand.Seed(time.Now().UnixNano())
	color := colors[rand.Intn(len(colors))]
	fmt.Printf("%s\033", color)
	// escape sequence consumes first character of input, provided zero-width space. if this line is deleted, the first character of input will be consumed.
	fmt.Printf("​")
	inputReader := bufio.NewReader(os.Stdin)
	outputWriter := bufio.NewWriter(os.Stdout)

	io.Copy(outputWriter, inputReader)
	// reset terminal color
	fmt.Print("\033[0m")
}

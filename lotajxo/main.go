package main

import (
	"fmt"
	"math/rand/v2" // Import the updated rand/v2 package
	"os"
)

var (
	CommitId string
	Version  string
	Revision = ".0"
)

const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[1;31m" // Bright Red
	ColorGreen   = "\033[1;32m" // Bright Green
	ColorMagenta = "\033[1;35m" // Bright Magenta
	ColorCyan    = "\033[1;36m" // Bright Cyan
)

.
func generateAndReflectRandomUint8V2() {
	randomNum := uint8(rand.IntN(256))

	fmt.Printf("Generated random uint8: %s%d%s (Decimal) | %s0x%x%s (Hex) | %s0b%08b%s (Binary)\n",
		ColorGreen, randomNum, ColorReset, // Decimal in Green
		ColorCyan, randomNum, ColorReset, // Hex in Cyan
		ColorMagenta, randomNum, ColorReset) // Binary in Magenta
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Printf("lotajxo %s%s (%s)\n", Version, Revision, CommitId)
		return
	} else if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "help") {
		// TODO: Add help message
		fmt.Printf("random uint8 generator simultanious representations in decimal, hexadecimal, and binary\n\t --version, -v, version\n\t --help, -h, this help message\n")
	} else if len(os.Args) > 1 {
		fmt.Printf("%sunrecognized arguments:%s %s\n", ColorRed,os.Args[1:], ColorReset)
	}
	generateAndReflectRandomUint8V2()
}

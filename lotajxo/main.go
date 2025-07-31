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

// ANSI escape codes for neon bright colors
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[1;31m" // Bright Red
	ColorGreen   = "\033[1;32m" // Bright Green
	ColorYellow  = "\033[1;33m" // Bright Yellow
	ColorBlue    = "\033[1;34m" // Bright Blue
	ColorMagenta = "\033[1;35m" // Bright Magenta
	ColorCyan    = "\033[1;36m" // Bright Cyan
)

// generateAndReflectRandomUint8V2 generates a random uint8 and prints its
// decimal, hexadecimal, and binary representations using math/rand/v2.
func generateAndReflectRandomUint8V2() {
	// Generate a random uint8.
	// rand.IntN(256) returns a non-negative pseudo-random number in the half-open interval [0, 256).
	// Casting it to uint8 ensures it fits within the 0-255 range.
	randomNum := uint8(rand.IntN(256))

	// Print all representations on a single line with ANSI colorization.
	// Each representation gets a different bright color.
	// ColorReset is used to reset the color after each part to avoid bleeding into the next.
	fmt.Printf("Generated random uint8: %s%d%s (Decimal) | %s0x%x%s (Hex) | %s0b%08b%s (Binary)\n",
		ColorGreen, randomNum, ColorReset, // Decimal in Green
		ColorCyan, randomNum, ColorReset, // Hex in Cyan
		ColorMagenta, randomNum, ColorReset) // Binary in Magenta
}

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "notAFlag")
	}
	switch os.Args[1] {
	case "--version", "-v", "version":
		fmt.Printf("%s%s (%s)\n", Version, Revision, CommitId)
		return
	default:
		generateAndReflectRandomUint8V2()
	}
}

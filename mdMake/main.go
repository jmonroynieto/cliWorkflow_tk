package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	Version  string
	Revision = ".0"
	CommitID string
)

func main() {
	//flag for version -v
	PrintVersion := flag.Bool("v", false, "version")
	flag.BoolVar(PrintVersion, "version", false, "version")
	flag.Parse()
	if *PrintVersion {
		fmt.Println("Version:", Version, " CommitID:", CommitID)
		os.Exit(0)
	}
	// Read input from stdin
	scanner := bufio.NewScanner(os.Stdin)
	var lines [][]string

	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, strings.Split(line, "\t"))
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading standard input:", err)
		os.Exit(1)
	}

	// Determine the maximum width of each column
	if len(lines) == 0 {
		fmt.Fprintln(os.Stderr, "No data provided")
		os.Exit(1)
	}

	columnWidths := make([]int, len(lines[0]))
	for _, line := range lines {
		for i, cell := range line {
			if len(cell) > columnWidths[i] {
				columnWidths[i] = len(cell)
			}
		}
	}

	// Print the markdown table
	printRow := func(row []string) {
		for i, cell := range row {
			fmt.Printf("| %s ", centerText(cell, columnWidths[i]))
		}
		fmt.Println("|")
	}

	printSeparator := func() {
		for _, width := range columnWidths {
			fmt.Print("|")
			fmt.Print(strings.Repeat(" ", (width/2)+2))
			fmt.Print(":")
			fmt.Print(strings.Repeat("-", (width/2)+1))
			fmt.Print(":")
			fmt.Print(strings.Repeat(" ", (width/2)+1))
		}
		fmt.Println("|")
	}

	// Print header
	printRow(lines[0])
	printSeparator()

	// Print the rest of the table
	for _, line := range lines[1:] {
		printRow(line)
	}
}

func centerText(text string, width int) string {
	padding := width - len(text)
	leftPadding := padding / 2
	rightPadding := padding - leftPadding
	return strings.Repeat(" ", leftPadding) + text + strings.Repeat(" ", rightPadding)
}

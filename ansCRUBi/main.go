package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
)

// TODO: manage file overwrite instead of printout

func main() {
	regex := regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

	if len(os.Args) == 1 {
		cleanLines(os.Stdin, regex)
	} else {
        if os.Args[1]== "-" {
            cleanLines(os.Stdin, regex)
            return
        }
		for _, filename := range os.Args[1:] {
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: file %s doesn't exist\n", filename)
				continue
			}

			file, err := os.Open(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", filename, err)
				continue
			}
			defer file.Close()

			cleanLines(file, regex)
		}

	}
}

func cleanLines(r io.Reader, regex *regexp.Regexp) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		cleanedLine := regex.ReplaceAllString(line, "")
		fmt.Println(cleanedLine)
	}
}

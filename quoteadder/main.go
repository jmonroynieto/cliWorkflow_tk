package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pydpll/errorutils"
)

var global []byte

var (
	Version  string
	Revision = ".0"
	CommitId string
)
var versionString = fmt.Sprintf("quoteadder version %s%s (%s)", Version, Revision, CommitId)

func main() {
	// Check that the user has provided a filename.
	if len(os.Args) < 2 {
		fmt.Println("Please provide a filename.")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "-h", "--help":
		fmt.Println(versionString)
		fmt.Println("Usage: quoteadder <filename>")
		os.Exit(0)
	case "-v", "--version":
		fmt.Println(versionString)
		os.Exit(0)
	}
	// use flags to determine if the user wants to use normally and use the first argument or use debug mode
	var debug *bool = flag.Bool("debug", false, "use mock data instead of user input")
	flag.Parse()
	// Get the JSON file from the command line.
	scn := bufio.NewScanner(os.Stdin)
	if *debug {
		// use mock data from bytes [h=104, m=109, y=121, ctl-]=29, r=114, t=116, y=121, 29, 110] into scn
		bytes := []byte{
			104, 10, 109, 10, 121, 10, // hmy
			29, 10, // ctl-]
			114, 10, 116, 10, 121, 10, 84, 10, // rtyT
			29, 10, // ctl-]
			121, 10, // y
			104, 10, 109, 10, 121, 10, // hmy
			29, 10, // ctl-]
			114, 10, 116, 10, 121, 10, // rty
			29, 10, // ctl-]
			110, 10, // n
		}
		scn = bufio.NewScanner(strings.NewReader(
			// mock data
			string(bytes)))
		// print current buffer of scn as string without consuming it
		fmt.Println(bytes, '\n', string(bytes))
	}
	filename := flag.Args()[0]
	// call go routine that checks for interrupt signals in case the user wants to quit
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func(sigs chan os.Signal) {
		// create a ticker
		ticker := time.NewTicker(time.Second / 2)
		// loop until the ticker is stopped
		for {
			select {
			case <-sigs:
				os.Exit(0)
			case <-ticker.C:
				// do nothing
			}
		}
	}(sigs)

	// Open the file and read the JSON data.
	file := mustOpen(filename)
	defer file.Close()
	var err error
	var data []map[string]string
	err = json.NewDecoder(file).Decode(&data)
	if err != nil {
		if err.Error() == "EOF" {
		} else {
			fmt.Println(err)
			return
		}
	}
	var anotherQuote bool = true
	// prompt user to enter a quote and attribution\
	for anotherQuote {
		var quote, attribution string
		quote = readFromUser_multiline("Enter a quote:", scn)
		quote = strings.TrimSpace(quote)
		// scan multiline input
		attribution = readFromUser_multiline("Enter an attribution:", scn)
		attribution = strings.TrimSpace(attribution)
		anotherQuote = boolyn("Add another quote? (y/n)", scn)
		// add quote and attribution to data
		data = append(data, map[string]string{"text": quote, "attribution": attribution})
	}
	// Write the JSON data back to the file with replaced data. overwrites the file
	err = file.Truncate(0)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = json.NewEncoder(file).Encode(data)
	if *debug {
		// print global as byte sequence
		fmt.Println(global)
		// print data structure
		fmt.Printf("%+v\n", data)
	}
	if err != nil {
		fmt.Println(err)
		return
	}
}

// mustOpen opens the file with the given name. If the file cannot be opened,
// the file is created and eturned as an os.File pointer.
func mustOpen(name string) *os.File {
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o644)
	errorutils.ExitOnFail(err)
	return file
}

func readFromUser_multiline(prompt string, scn *bufio.Scanner) string {
	fmt.Println(prompt)
	var lines []string
	for scn.Scan() {
		line := scn.Text()
		if len(line) == 1 {
			// Group Separator (GS ^]): ctrl-]
			if line[0] == '\x1D' {
				line = line + "\n"
				global = append(global, line[0])
				break
			}
		}
		lineasbytes := []byte(line)
		lineasbytes = append(lineasbytes, '\n')
		global = append(global, lineasbytes...)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// boolyn prompts the user for a yes or no answer and returns true or false, handles long form and short form answers in any capitalization
func boolyn(prompt string, scn *bufio.Scanner) bool {
	var answer string
	for {
		fmt.Println(prompt)
		scn.Scan()
		answer = scn.Text()
		answer = strings.ToLower(answer)
		global = append(global, []byte(answer+"\n")...)
		switch answer {
		case "yes", "y":
			return true
		case "no", "n":
			return false
		default:
			fmt.Println("Please answer yes or no.")
		}
	}
}

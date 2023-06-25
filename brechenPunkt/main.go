// brechenPunkt is a simple tool to create logpoints for vscode via BreakpointIO extension
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
)

type line struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Punkt struct {
	Location   string `json:"location"`
	Lines      []line `json:"line"`
	Enabled    bool   `json:"enabled"`
	LogMessage string `json:"logMessage,omitempty"`
}
type collection []Punkt

func shouldContinue() bool {
	test := promptui.Prompt{
		Label:     "Continue?",
		IsConfirm: true,
		Default:   "y",
		Validate: func(input string) error {
			response := strings.ToLower(input)
			if response == "y" || response == "n" || response == "yes" || response == "no" {
				return nil
			}
			//reset the input of promptui
			return fmt.Errorf("invalid input: backspace and try again")
		},
	}
	r, err := test.Run()
	if err != nil && err.Error() != "" {
		if err.Error() == "invalid input" {
			return shouldContinue()
		} else {
			fmt.Printf("Prompt failed %v\n", err)
			return false
		}
	}
	if strings.ToLower(r) == "y" || strings.ToLower(r) == "yes" {
		return true
	}
	return false
}

var message = promptui.Prompt{
	Label:       "\x1b[32mEnter <location>:<message>\x1b[0m",
	HideEntered: true,
}

func main() {
	outputfilename := "breakpoints.json"
	var f *os.File

	//annotee is required
	if len(os.Args) < 2 {
		fmt.Println("filename is required")
		os.Exit(1)
	}
	annotee := os.Args[1]
	if _, err := os.Stat(annotee); os.IsNotExist(err) {
		fmt.Println("the file you are trying to annotate does not exist")
		os.Exit(1)
	}
	var fErr error
	permission := os.O_RDWR | os.O_CREATE
	if _, err := os.Stat(".vscode"); os.IsNotExist(err) {
		//stat the current directory file
		if _, err := os.Stat(outputfilename); os.IsNotExist(err) {
			f, fErr = os.Create(outputfilename)
		} else {
			f, fErr = os.OpenFile(outputfilename, permission, 0644)
		}
	} else {
		if _, err = os.Stat(".vscode/breakpoints.json"); os.IsNotExist(err) {
			f, fErr = os.Create(".vscode/breakpoints.json")
		} else {
			f, fErr = os.OpenFile(".vscode/breakpoints.json", permission, 0644)
		}
	}
	if fErr != nil {
		fmt.Println("error accessing breakpoints.json _ ", fErr)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Println("\x1b[34mSkip message to create regular breakpoint\nCtl+C resets the prompt\nEntring Ctl+']' will terminate and save \x1b[0m\n****")
	//get the filename without the path
	//split on the last slash
	var oldCounter, newCounter int
	var c collection
	var l Punkt

	//read all the breakpoint file into dat
	dat, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	//unmarshal the json
	err = json.Unmarshal(dat, &c)
	if err != nil {
		if err.Error() == "unexpected end of JSON input" && len(c) == 0 {
			fmt.Println("no breakpoints to read")
			c := shouldContinue()
			if !c {
				os.Exit(0)
			}
		} else {
			fmt.Printf("error: %v\n", err)
			os.Exit(1)
		}
	}
	oldCounter = len(c)
	//set to root of workspace
	//..if annotee has '/' prefix remove it
	annotee = strings.TrimPrefix(annotee, "./")
	l.Location = fmt.Sprintf("/%s", annotee)
	for {
		text, err := message.Run()
		if err != nil {
			fmt.Printf("unexpected error %v", err)
			continue
		}
		//when input is control character trl-] exit
		if strings.Contains(text, "\x1d") {
			fmt.Printf("\x1b[34mYou inputed %d new logpoints to your previous %d!\x1b[0m\n", newCounter, oldCounter)
			break
		}
		text = strings.TrimSpace(text)
		split := strings.Split(text, ":")
		lineNo, err := strconv.Atoi(strings.TrimSpace(split[0]))
		if err != nil {
			fmt.Printf("\x1b[31m\tbad input – try again\x1b[0m\n")
			//reset ansii color
			fmt.Printf("\x1b[0m")
			fmt.Printf("Failed – %s\n", text)
			continue
		}

		//set all attributes of the punkt
		//message is a whitespace stripped version of the input text
		if len(split) < 2 || strings.TrimSpace(split[1]) == "" {
			l.LogMessage = ""
		} else {
			l.LogMessage = text
		}
		l.Lines = []line{{lineNo - 1, 0}, {lineNo - 1, 0}}
		l.Enabled = true
		c = append(c, l)
		fmt.Printf("Entered – %s\n", strings.Trim(text, ":"))
		//reset
		l.Lines = nil
		l.LogMessage = ""
		newCounter++
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	if len(b) < 20 {
		fmt.Printf("\x1b[31m\tNo breakpoints to write, exiting\x1b[0m\n")
		os.Exit(0)
	}
	//overwrite the file
	f.Truncate(0)
	f.Seek(0, 0)
	f.Write(b)
}

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type Line struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type punkt struct {
	Location   string `json:"location"`
	Lines      []Line `json:"line"`
	Enabled    bool   `json:"enabled"`
	LogMessage string `json:"logMessage"`
}

type collection []punkt

func main() {
	outputfile := "breakpoints.json"
	//get argument from command line, must be a real file on the run directory
	//if not exit with error
	//if it exists just keep the filename, no dir info
	// filename is required
	if len(os.Args) < 2 {
		fmt.Println("filename is required")
		os.Exit(1)
	}
	filename := os.Args[1]
	//check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Println("file does not exist")
		os.Exit(1)
	}
	//get the filename without the path
	//split on the last slash
	var oldCounter, newCounter int
	var c collection
	var l punkt
	reader := bufio.NewReader(os.Stdin)

	//read output file, and append its items to collection
	//if file does not exist, create it
	if _, err := os.Stat(outputfile); os.IsNotExist(err) {
		//create the file
		f, err := os.Create(outputfile)
		if err != nil {
			fmt.Println("error reating breakpoints.json")
			panic(err)
		}
		defer f.Close()
	} else {
		//read the file
		dat, err := ioutil.ReadFile(outputfile)
		if err != nil {
			panic(err)
		}
		//unmarshal the json
		err = json.Unmarshal(dat, &c)
		if err != nil {
			panic(err)
		}
	}
	oldCounter = len(c)
	l.Location = filename
	for {
		fmt.Printf("\x1b[32mEnter <location>:<message>\x1b[0m\n")
		text, _ := reader.ReadString('\n')
		//when input is control character trl-] exit
		if strings.Contains(text, "\x1d") {
			fmt.Printf("\x1b[34mYou inputed %d new logpoints to your previous %d!\x1b[0m\n", newCounter, oldCounter)
			break
		}
		split := strings.Split(text, ":")
		lineNo, err := strconv.Atoi(strings.TrimSpace(split[0]))
		if err != nil {
			fmt.Printf("\x1b[31m\tbad input – error converting line number to int: %v\x1b[0m\n", err)
			fmt.Printf("\x1b[31m\ttry again\x1b[0m\n")
			//reset ansii color
			fmt.Printf("\x1b[0m")
			fmt.Printf("your current message was\n %s:%s", split[0], text)
			continue
		}
		//set all attributes of the punkt
		//message is a whitespace stripped version of the input text
		l.LogMessage = strings.TrimSpace(text)
		l.Lines = []Line{{lineNo, 0}, {lineNo, 0}}
		l.Enabled = true
		c = append(c, l)
		l.Lines = nil
		l.LogMessage = ""
		newCounter++
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	ioutil.WriteFile(outputfile, b, 0644)
}

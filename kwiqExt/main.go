package main

import (
	"fmt"
	"os"
	"strings"

	filetyper "kwiqExt/fileTyper"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
)

var (
	Version  = "0.1"
	CommitId string
)

func main() {
	if errorutils.ToggleColor() { // ensure color output is disabled
		errorutils.ToggleColor()
	}
	//cli flags
	if len(os.Args) < 2 {
		fmt.Println("Usage: kwiqExt [-d] <file1> <file2> ...")
		os.Exit(1)
	}
	index := 1
	if os.Args[1] == "-d" {
		logrus.SetLevel(logrus.DebugLevel)
		index++
	}
	if os.Args[1] == "--version" {
		fmt.Printf("%s (%s)\n", Version, CommitId)
		if len(os.Args) > 2 {
			logrus.Error("Unused arguments: " + strings.Join(os.Args[2:], " "))
		}
	}

	for _, file := range os.Args[index:] {
		f, err := filetyper.DetermineFMTtype(file)
		errorutils.ExitOnFail(err)
		fmt.Printf("file %s was detected as %s\n", file, f)
	}
}

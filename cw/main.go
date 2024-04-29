package main

//cw is a program designed to add bookmarks to the terminal for quick access to files and folders.
//cw is written in go and implements cw set and cw unset for any alphanumetic 1 character key. The bookmarks are saved in a file called ~/.cw_bookmarks.sh which is a bash file with the cw command definitions as aliases such that one can use cw1 to cd to the command saved in the register 1 and cwb to cd to de bookmark in the register b. The aliases use cx as the cd command; this command is an implentation of 'cd $new; clear; ls'.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pydpll/errorutils"
)

var (
	Version  = "1.1.1"
	CommitId string
)

func main() {
	//get the arguments
	args := os.Args
	if len(args) == 1 {
		printHelp()
		return
	}
	args = args[1:]
	//get the command
	command := args[0]
	//execute command
	if command == "set" {
		set(args[1:])
	} else if command == "unset" {
		unset(args[1:])
	} else if command == "list" {
		list()
	} else if command == "version" {
		fmt.Printf("Version: %s - %s\n", Version, CommitId)
	} else {
		printHelp()
	}
}

func set(args []string) {
	if len(args) < 2 {
		fmt.Printf("Error: not enough arguments given\n")
		fmt.Printf("command was cw set %s\n\n", args)
		printHelp()
		return
	}

	key := args[0]
	newPath := args[1]
	aliases := readAliases()
	bookmarks, keys := extractBM(aliases)
	//check if the key is already in use
	if _, ok := bookmarks[key]; ok {
		fmt.Printf("Warning: replacing bookmark %s. Old path: %s, new path: %s\n", key, bookmarks[key], newPath)
	} else {
		fmt.Printf("Bookmark %s set to %s\n", key, newPath)
	}
	bookmarks[key] = newPath
	keys = append(keys, key)
	slices.Sort(keys)
	WriteFile(slices.Compact(keys), bookmarks)
}

// unset unsets the bookmark
func unset(request []string) {

	if len(request) == 0 {
		fmt.Printf("Error: no items to delete\n")
		printHelp()
		return
	}

	aliases := readAliases()
	bookmarks, keys := extractBM(aliases)
	var dest string
	for _, key := range request {
		dest += bookmarks[key] + " "
		delete(bookmarks, key)
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	for _, key := range request {
		keys = remove(keys, key)
	}
	WriteFile(keys, bookmarks)
	fmt.Printf("Bookmarks matching the requested registers have been deleted\nRequested: %s\nLocations Removed: %s\n", request, dest)
}

// list lists all the bookmarks
func list() {
	aliases := readAliases()
	bookmarks, keys := extractBM(aliases)
	slices.Sort(keys)
	for _, key := range slices.Compact(keys) {
		fmt.Printf("%s\t->\t%s\n", key, bookmarks[key])
	}
}

// printHelp prints the help
func printHelp() {
	fmt.Printf("Version: %s - %s\n", Version, CommitId)
	fmt.Println(`Creates aliases for bookmarks (e.g., cw1, cwb) for easy access

**Description:**

cw enables you to create convenient bookmarks for frequently accessed folders within your terminal, allowing for quick and easy navigation.

**Usage:**

- **Set a bookmark:**
 cw set <key> <path>
  - Example: cw set d ~/Documents

- **Remove a bookmark:**
 cw unset <key>
  - Example: cw unset d

- **List all bookmarks:**
 cw list

**Additional Features:**

- **Backward compatibility:**
 'setP' and 'showP' commands are included for compatibility with earlier cw versions.

**Technical Details:**

- Stores bookmarks in a Bash file: ~/.cw_bookmarks.sh

**Start bookmarking your favorite directories and boost your terminal productivity with cw!**

** ** ** **`)
}

// extract info from alias declarations
func extractBM(lines []string) (map[string]string, []string) {
	bookmarks := make(map[string]string)
	keys := make([]string, 0)
	for _, line := range lines {
		split := strings.Split(line, "=")
		roughKey := strings.TrimSpace(split[0])
		roughPath := strings.ReplaceAll(split[1], "cx", "")
		roughPath = strings.ReplaceAll(roughPath, "'", "")
		key := strings.ReplaceAll(roughKey, "alias cw", "")
		path := strings.TrimSpace(roughPath)
		//add to map
		bookmarks[key] = path
		keys = append(keys, key)

	}
	return bookmarks, keys
}

func openFile(flag int) *os.File {
	//get the home directory
	home, err := os.UserHomeDir()
	errorutils.ExitOnFail(err,
		errorutils.WithLineRef("XXKbyHh7KBI"),
		errorutils.WithMsg("Error getting home directory"),
	)
	//get the path to the file
	pathToFile := filepath.Join(home, ".cw_bookmarks.sh")

	//open the file
	file, err := os.OpenFile(pathToFile, flag, 0644)
	errorutils.ExitOnFail(err)
	return file
}

func readAliases() []string {
	file := openFile(os.O_RDONLY)
	//read the file
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	file.Close()
	// aliases capture
	aliases := make([]string, 0)
	for _, line := range lines {
		//if the line does not contain the key, add it to the new lines
		if strings.HasPrefix(line, "alias") {
			aliases = append(aliases, line)
		}
	}
	return aliases
}

func giveHeader() string {
	return `#!/bin/bash
# This file was generated by cw and is used to store the bookmarks
# see github.com/jmonroynieto/cw for more information
# To use the bookmarks, source this file in your .bashrc or .zshrc

setP() { loc=${2:-$(pwd)}; cw set ${1} ${loc}; source ~/.cw_bookmarks.sh ; }
unsetP() { cw unset ${@}; for key in ${@}; do unalias cw${key} ; done ; }
showP() { cw list; }


`
}

func WriteFile(keys []string, bookmarks map[string]string) {
	file := openFile(os.O_APPEND | os.O_CREATE | os.O_RDWR | os.O_TRUNC)
	file.Truncate(0)
	file.WriteString(giveHeader())
	for _, k := range keys {
		file.WriteString(fmt.Sprintf("alias cw%.1s='cx %s'\n", k, bookmarks[k]))
	}
	file.Close()
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

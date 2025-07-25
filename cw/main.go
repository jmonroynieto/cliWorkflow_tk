package main

// cw is a program designed to add bookmarks to the terminal for quick access to files and folders.
// cw is written in go and implements cw set and cw unset for any alphanumetic 1 character key. The bookmarks are saved in a file called ~/.cw_bookmarks.sh which is a bash file with the cw command definitions as aliases such that one can use cw1 to cd to the command saved in the register 1 and cwb to cd to de bookmark in the register b. The aliases use cx as the cd command; this command is an implentation of 'cd $new; clear; ls'.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

var (
	Version  string
	Revision = ".0"
	CommitId string
)

var app = cli.Command{
	Name:                          "cw",
	Description:                   "cw is a program designed to add bookmarks to the terminal for quick access to files and folders",
	Commands:                      appCmds,
	Version:                       fmt.Sprintf("%s%s (%s)", Version, Revision, CommitId),
	CustomRootCommandHelpTemplate: printHelp,
}

var appCmds = []*cli.Command{
	{
		Name:               "set",
		Usage:              "set a bookmark",
		Action:             set,
		ArgsUsage:          "<key> <location>",
		CustomHelpTemplate: subcmdHelp,
	},
	{
		Name:               "unset",
		Usage:              "unset a bookmark",
		Action:             unset,
		ArgsUsage:          "<key>",
		CustomHelpTemplate: subcmdHelp,
	},
	{
		Name:               "list",
		Usage:              "list all bookmarks",
		Action:             list,
		ArgsUsage:          "",
		CustomHelpTemplate: subcmdHelp,
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func set(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) < 2 {
		fmt.Printf("Error: not enough arguments given\n")
		fmt.Printf("command was cw set %s\n\n", args)
		fmt.Println(cmd.Usage)
		return nil
	}

	key := args[0]
	newPath := args[1]
	aliases := readAliases()
	bookmarks, keys := extractBM(aliases)
	// check if the key is already in use
	if _, ok := bookmarks[key]; ok {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Warning: overwriting cw%s pick which one to keep\n", key)).
					Options(
						huh.NewOption(bookmarks[key], bookmarks[key]),
						huh.NewOption(newPath, newPath),
					).
					Value(&newPath),
			))
		form.WithTheme(huh.ThemeBase())
		err := form.Run()
		errorutils.WarnOnFail(err)
	} else {
		fmt.Printf("Bookmark %s set to %s\n", key, newPath)
	}
	bookmarks[key] = newPath
	keys = append(keys, key)
	slices.Sort(keys)
	WriteFile(slices.Compact(keys), bookmarks)
	return nil
}

// unset unsets the bookmark
func unset(ctx context.Context, cmd *cli.Command) error {
	request := cmd.Args().Slice()
	if len(request) == 0 {
		return errors.New("error: no items to delete")
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
	return nil
}

// list lists all the bookmarks
func list(ctx context.Context, cmd *cli.Command) error {
	aliases := readAliases()
	bookmarks, keys := extractBM(aliases)
	slices.Sort(keys)
	for _, key := range slices.Compact(keys) {
		fmt.Printf("%s\t->\t%s\n", key, bookmarks[key])
	}
	return nil
}

// printHelp prints the help
var printHelp string = `{{"\033[1m"}}{{.Name}}{{"\033[0m"}} - {{.Description}}
{{ "Create convenient bookmarks for frequently accessed folders within your terminal, allowing for quick and easy navigation." }}
		 - Keys must be single characters. They will create aliases as 'cw<key> (e.g. cw1, cw2, cwO)'
		 - Keys are case sensitive
		 - Stores bookmarks in a Bash file: ~/.cw_bookmarks.sh
		 - The 'setP', 'unsetP', and 'showP' commands are set as aliases for quick operation.

USAGE:
 	{{.Name}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{ if .ArgsUsage}} {{.ArgsUsage}}{{else}}[arguments...]{{end}}

COMMANDS:
{{range .Commands}}{{if not .HideHelp}}   {{join .Names ", "}}{{" "}}{{.ArgsUsage}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}{{if .Version}}
VERSION: {{.Version}}{{end}}
`

var subcmdHelp string = `NAME: {{.Name}} - {{.Usage}}
cw {{.HelpName}} {{if .ArgsUsage}}{{.ArgsUsage}} {{if .VisibleFlags}}[global options]{{end}}
{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`

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
		// add to map
		bookmarks[key] = path
		keys = append(keys, key)

	}
	return bookmarks, keys
}

func openFile(flag int) *os.File {
	// get the home directory
	home, err := os.UserHomeDir()
	errorutils.ExitOnFail(err,
		errorutils.WithLineRef("XXKbyHh7KBI"),
		errorutils.WithMsg("Error getting home directory"),
	)
	// get the path to the file
	pathToFile := filepath.Join(home, ".cw_bookmarks.sh")

	// open the file
	file, err := os.OpenFile(pathToFile, flag, 0o644)
	errorutils.ExitOnFail(err)
	return file
}

func readAliases() []string {
	file := openFile(os.O_RDONLY)
	// read the file
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
		// if the line does not contain the key, add it to the new lines
		if strings.HasPrefix(line, "alias") {
			aliases = append(aliases, line)
		}
	}
	return aliases
}

func giveHeader() string {
	return `#!/bin/bash
# This file was generated by cw and is used to store the bookmarks
# see github.com/jmonroynieto/cliWorkflow_tk/cw for more information
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

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	Version  = "1.1.0"
	CommitId string
)

func main() {
	regex := regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

	app := &cli.App{
		Name:    "ansCRUBi",
		Usage:   "Removes ansi control characters maybe left over from colorized commands",
		Flags:   appFlags,
		Version: fmt.Sprintf("%s - %s", Version, CommitId),
		Action: func(ctx *cli.Context) error {
			argLen := ctx.Args().Len()
			if a := ctx.Args().First(); !ctx.Bool("files") && (a == "-" || a == "") {
				cleanLines(os.Stdin, regex, os.Stdout)
			} else if !ctx.Bool("files") && argLen > 0 {
				cli.ShowAppHelp(ctx)
				return errorutils.NewReport("ERROR: unknown positional arguments", "m700KwVadVJ")
			}
			for _, filename := range ctx.Args().Slice() {
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					errorutils.WarnOnFail(err)
					continue
				}

				file, err := os.Open(filename)
				if err != nil {
					errorutils.WarnOnFail(err)
					continue
				}
				defer file.Close()
				var w io.Writer = os.Stdout
				if ctx.Bool("overwrite") {
					w = file
				}

				cleanLines(file, regex, w)
			}

			return nil
		},
	}

	app.Run(os.Args)
}

var appFlags []cli.Flag = []cli.Flag{
	&cli.BoolFlag{
		Name:    "debug",
		Aliases: []string{"d"},
		Usage:   "activates debugging messages",
		Action: func(ctx *cli.Context, shouldDebug bool) error {
			if shouldDebug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil

		},
	},
	&cli.BoolFlag{
		Name:    "files",
		Aliases: []string{"f"},
		Usage:   "arguments are `PATHS...` to files to change",
	},
	&cli.BoolFlag{
		Name:    "overwrite",
		Aliases: []string{"o"},
		Usage:   "explicitly change files, if not set -f will print to stdin",
	},
}

func cleanLines(r io.Reader, regex *regexp.Regexp, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		cleanedLine := regex.ReplaceAllString(line, "")
		fmt.Fprintln(w, cleanedLine)
	}
}

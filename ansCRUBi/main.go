package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version  string
	Revision = ".0" //.x
	CommitId string
)

func main() {
	regex := regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

	app := &cli.Command{
		Name:      "ansCRUBi",
		UsageText: "ansCRUBi [-o] [-f files...]",
		Usage:     "Removes ansi control characters left over from colorized commands",
		Flags:     appFlags,
		Version:   fmt.Sprintf("%s (%s)", Version+Revision, CommitId),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// piping only
			if a := cmd.Args().First(); !cmd.IsSet("files") && (a == "-" || a == "") {
				cleanLines(os.Stdin, regex, os.Stdout)
			} else if cmd.Args().Len() > 0 && !cmd.Bool("files") {
				return errorutils.NewReport(fmt.Sprintf("ERROR: unknown arguments: %q", cmd.Args()), "m700KwVadVJ")
			} else if len(cmd.StringSlice("files")) == 0 {
				return errorutils.NewReport("ERROR: no files provided", "VLsmXBwQrya")
			}
			for _, filename := range cmd.StringSlice("files") {
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					errorutils.WarnOnFail(err, errorutils.WithMsg("origin file "+filename+" does not exist"))
					continue
				}
				//open file with permission to overwrite
				origin, err := os.OpenFile(filename, os.O_RDWR, 0755)
				if err != nil {
					errorutils.WarnOnFail(err)
					continue
				}
				var w = os.Stdout
				if cmd.Bool("overwrite") {
					w = origin
				}

				cleanLines(origin, regex, w)
			}

			return nil
		},
	}

	app.Run(context.Background(), os.Args)
}

var appFlags []cli.Flag = []cli.Flag{
	&cli.BoolFlag{
		Name:    "debug",
		Aliases: []string{"d"},
		Usage:   "activates debugging messages",
		Action: func(ctx context.Context, cmd *cli.Command, shouldDebug bool) error {
			if shouldDebug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil

		},
	},
	&cli.StringSliceFlag{
		Name:    "files",
		Aliases: []string{"f"},
		Usage:   "`PATHS...` to files to change",
	},
	&cli.BoolFlag{
		Name:    "overwrite",
		Aliases: []string{"o"},
		Usage:   "explicitly change files, if not set will print to stdout",
	},
}

func cleanLines(r io.ReadCloser, regex *regexp.Regexp, w *os.File) error {
	var ww *bufio.Writer
	scanner := bufio.NewScanner(r)
	if w != os.Stdout {
		temp, err := os.CreateTemp("", "ansCRUBi*")
		if err != nil {
			return err
		}
		ww = bufio.NewWriter(temp)
		defer overwrite(temp, w)
	} else {
		ww = bufio.NewWriter(w)
		defer errorutils.NotifyClose(r)
	}
	defer ww.Flush()

	for scanner.Scan() {
		line := scanner.Text() // reading lines to avoid truncating matches at read tail
		cleanedLine := regex.ReplaceAllString(line, "")
		fmt.Fprintln(ww, cleanedLine)
	}
	return nil
}

func overwrite(temp *os.File, w *os.File) {
	//quickly replace contents of w with contents of temp
	err := w.Truncate(0)
	errorutils.WarnOnFail(err, errorutils.WithMsg("failed to truncate file"), errorutils.WithLineRef("xWOlo5XlUtO"))
	_, err = w.Seek(0, io.SeekStart)
	errorutils.ExitOnFail(err, errorutils.WithMsg("failed to seek file"), errorutils.WithLineRef("VSB4YGUFjm9"))
	_, err = temp.Seek(0, io.SeekStart)
	errorutils.ExitOnFail(err, errorutils.WithMsg("failed to seek file"), errorutils.WithLineRef("3sjh4Njxt45"))
	//read and copy into
	_, err = io.Copy(w, temp)
	errorutils.ExitOnFail(err, errorutils.WithMsg("failed to copy file"), errorutils.WithLineRef("eZUXo2Erji5"))
	err = w.Sync()
	errorutils.ExitOnFail(err, errorutils.WithMsg("failed to sync file"), errorutils.WithLineRef("Fbtv735kBGn"))
	errorutils.NotifyClose(temp)
	errorutils.NotifyClose(w)
	os.Remove(temp.Name())
}

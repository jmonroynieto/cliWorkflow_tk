package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	Version    = "0.1"
	CommitId   string
	ignorable  []*regexp.Regexp
	workersNum int
)

func main() {
	sort.Sort(cli.FlagsByName(appFlags))

	app := &cli.App{
		Name:    "indexFiles",
		Usage:   "recursive, parallel sha1sum for files and symlinks in directory",
		Flags:   appFlags,
		Version: fmt.Sprintf("%s - %s", Version, CommitId),
		Action: func(ctx *cli.Context) error {
			run(ctx.String("examine"), ctx.String("output"))
			return nil
		},
	}

	err := app.Run(os.Args)
	errorutils.WarnOnFail(err, errorutils.WithMsg("app failed execution"))
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
	&cli.StringFlag{
		Name:     "examine",
		Aliases:  []string{"e", "i"}, //input
		Usage:    "`DIR` to examine",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "output",
		Aliases:  []string{"o"},
		Usage:    "`FILE` where the index should be saved to",
		Required: true,
	},
	&cli.StringFlag{
		Name:    "ignoreRegexes",
		Aliases: []string{"x"},
		Usage:   "`FILE` listing regex filenames to ignore",
		Action: func(ctx *cli.Context, path string) error {
			var e error
			ignorable, e = getIgnoreRegexes(path)
			return e
		},
	},
	&cli.IntFlag{
		Name:        "workers",
		Aliases:     []string{"w"},
		Destination: &workersNum,
		Value:       20,
		Hidden:      true,
	},
}

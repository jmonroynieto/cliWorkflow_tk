package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version    string
	Revision   = ".0"
	CommitId   string
	ignorable  []*regexp.Regexp
	workersNum int64
)

func main() {
	sort.Sort(cli.FlagsByName(appFlags))

	app := &cli.Command{
		Name:    "indexFiles",
		Usage:   "recursive, parallel sha1sum for files and symlinks in directory",
		Flags:   appFlags,
		Version: fmt.Sprintf("%s%s (%s)", Version, Revision, CommitId),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			run(cmd.String("examine"), cmd.String("output"))
			return nil
		},
	}

	err := app.Run(context.Background(), os.Args)
	errorutils.WarnOnFail(err, errorutils.WithMsg("app failed execution"))
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
	&cli.StringFlag{
		Name:     "examine",
		Aliases:  []string{"e", "i"}, // input
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
		Action: func(ctx context.Context, cmd *cli.Command, path string) error {
			var e error
			ignorable, e = getIgnoreRegexes(path)
			return e
		},
	},
	&cli.Int64Flag{
		Name:        "workers",
		Aliases:     []string{"w"},
		Destination: &workersNum,
		Value:       20,
		Hidden:      true,
	},
}

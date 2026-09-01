package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version  string
	Revision = "0"
	CommitId string
)

var debugFlag = &cli.BoolFlag{
	Name:    "debug",
	Aliases: []string{"D"},
	Usage:   "activates debugging messages",
	Action: func(ctx context.Context, cmd *cli.Command, shouldDebug bool) error {
		if shouldDebug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	},
}

var app = &cli.Command{
	Name:  "avaro",
	Usage: "report and toggle ideapad battery conservation mode",
	Description: "Conservation mode is an embedded-controller setting that stops charging near 80% to\n" +
		"slow calendar ageing of the cell. avaro with no subcommand reports whether it is\n" +
		"active right now and whether TLP is configured to keep it that way; avaro flip\n" +
		"switches it; avaro notes prints the commands that make a setting survive TLP.",
	Version: fmt.Sprintf("%s.%s (%s)", Version, Revision, CommitId),
	Flags:   []cli.Flag{debugFlag},
	Commands: []*cli.Command{
		flipCommand,
		notesCommand,
	},
	Action: statusAction,
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

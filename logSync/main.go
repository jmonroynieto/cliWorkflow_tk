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

var cfgPathFlag = &cli.StringFlag{
	Name:    "config",
	Aliases: []string{"c"},
	Usage:   "path to logSync config file (default ~/.config/logSync/config)",
}

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
	Name:    "logSync",
	Usage:   "reliable file sync between this machine and a phone over adb",
	Version: fmt.Sprintf("%s.%s (%s)", Version, Revision, CommitId),
	Flags:   []cli.Flag{cfgPathFlag, debugFlag},
	Commands: []*cli.Command{
		doctorCommand,
		syncCommand,
		pushCommand,
		pullCommand,
		notesyncCommand,
		snapshotCommand,
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

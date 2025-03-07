package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version  = "1.2.0"
	CommitId = ""
)

func main() {

	app := &cli.Command{
		Name:     "fickeFinger",
		Usage:    "custon random value generator",
		Flags:    appFlags,
		Commands: appCmds,
		Version:   fmt.Sprintf("%s (%s)", Version, CommitId),
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
}

var appCmds []*cli.Command = []*cli.Command{
	{
		Name:   "jitter",
		Usage:  "values between min and max with a minimum space between them",
		Action: generateJitter,
		Flags:  jitterFlags,
	},
	{
		Name:   "id",
		Usage:  "generate random id",
		Action: generateID,
		Flags:  idFlags,
	},
}

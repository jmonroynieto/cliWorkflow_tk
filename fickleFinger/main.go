package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	Version  = "0.1"
	CommitId = ""
)

func main() {

	app := &cli.App{
		Name:     "fickeFinger",
		Usage:    "custon random value generator",
		Flags:    appFlags,
		Commands: appCmds,
		Version:  fmt.Sprintf("%s - %s", Version, CommitId),
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

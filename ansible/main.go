package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitId string
)

var app = cli.Command{
	Name:        "Ansible",
	Description: "log simply log",
	Action:      superluminal,
	Version:     "v0.1.0 (" + CommitId + ")",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Action: func(c context.Context, cmd *cli.Command, debug bool) error {
				logrus.SetLevel(logrus.DebugLevel)
				return nil
			},
		},
		&cli.BoolFlag{
			Name:    "disable-color",
			Aliases: []string{"c"},
			Action: func(c context.Context, cmd *cli.Command, debug bool) error {
				if errorutils.ToggleColor() {
					errorutils.ToggleColor()
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:    "storage",
			Usage:   "`FILE` to log into. Default is ./ansible.log",
			Value:   "./ansible.log",
			Aliases: []string{"s"},
		},
		&cli.StringFlag{
			Name:    "level",
			Usage:   "log level. Default is info. possible values: debug, info, warn, error",
			Aliases: []string{"l"},
			Value:   "info",
		},
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func superluminal(ctx context.Context, cmd *cli.Command) error {
	// open file to append
	filename := cmd.String("storage")
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	errorutils.ExitOnFail(err)
	defer file.Close()

	logrus.SetOutput(file)
	switch cmd.String("level") {
	case "debug":
		logrus.Debug(strings.Join(cmd.Args().Slice(), " "))
	case "info":
		logrus.Info(strings.Join(cmd.Args().Slice(), " "))
	case "warn":
		logrus.Warn(strings.Join(cmd.Args().Slice(), " "))
	case "error":
		logrus.Error(strings.Join(cmd.Args().Slice(), " "))
	default:
		fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m\n", "no such log level "+cmd.String("level"))
	}
	return nil
}

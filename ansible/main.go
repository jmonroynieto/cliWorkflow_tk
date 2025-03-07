package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitId string
	Version = "v1.2.1"
)

var app = cli.Command{
	Name:        "Ansible",
	Description: "log simply log",
	Action:      superluminal,
	Version:   fmt.Sprintf("%s (%s)", Version, CommitId),
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
			Usage:   "`FILE` to log into.",
			Value:   "./ansible.log",
			Aliases: []string{"s"},
		},
		&cli.StringFlag{
			Name:    "level",
			Usage:   "log level. possible values: debug, info, warn, error",
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
	t := time.Now()
	// open file to append
	filename := cmd.String("storage")
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	errorutils.ExitOnFail(err)
	defer file.Close()
	lockFile(file)
	defer unlockFile(file)

	logrus.SetOutput(file)
	entry := logrus.NewEntry(logrus.StandardLogger())
	entry.Time = t //time of call not depending on mutex aquisition
	switch cmd.String("level") {
	case "debug":
		entry.Debug(strings.Join(cmd.Args().Slice(), " "))
	case "info":
		entry.Info(strings.Join(cmd.Args().Slice(), " "))
	case "warn":
		entry.Warn(strings.Join(cmd.Args().Slice(), " "))
	case "error":
		entry.Error(strings.Join(cmd.Args().Slice(), " "))
	default:
		fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m\n", "no such log level "+cmd.String("level"))
	}
	return nil
}

func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

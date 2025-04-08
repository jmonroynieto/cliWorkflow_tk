package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
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
	Version  string
	Revision = ".1"
)

var app = cli.Command{
	Name:        "Ansible",
	Description: "log simply log",
	Action:      superluminal,
	Version:     fmt.Sprintf("%s (%s)", Version+Revision, CommitId),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Action: func(c context.Context, cmd *cli.Command, debug bool) error {
				slog.SetLogLoggerLevel(slog.LevelDebug)
				return nil
			},
		},
		&cli.BoolFlag{
			Name:    "enable-color",
			Aliases: []string{"c"},
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
	Commands: []*cli.Command{
		{
			Name: "unlock",
			Action: func(c context.Context, cmd *cli.Command) error {
				logrus.Debug("unlocking file")
				filename := cmd.String("storage")
				file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0o644)
				errorutils.ExitOnFail(err)
				logrus.Debug("file connection stablished")
				return unlockFile(file)
			},
		},
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func superluminal(ctx context.Context, cmd *cli.Command) error {
	wantsColor := cmd.Bool("enable-color")
	if wantsColor != errorutils.ToggleColor() {
		errorutils.ToggleColor()
	}
	t := time.Now()
	slog.Debug("starting log")
	// open file to append
	filename := cmd.String("storage")
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	errorutils.ExitOnFail(err)
	defer func() {
		file.Close()
	}()
	lockFile(file)
	slog.Debug("locked file")
	defer unlockFile(file)

	logrus.SetOutput(file)
	entry := logrus.NewEntry(logrus.StandardLogger())
	entry.Time = t //time of call not depending on mutex aquisition

	var levelFunc func(args ...interface{})
	switch cmd.String("level") {
	case "debug":
		levelFunc = entry.Debug
	case "info":
		levelFunc = entry.Info
	case "warn":
		levelFunc = entry.Warn
	case "error":
		levelFunc = entry.Error
	default:
		fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m\n", "no such log level "+cmd.String("level"))
	}

	slog.Debug("program parameters set, moving on to processing input")
	msg := strings.Join(cmd.Args().Slice(), " ")
	//remove surrounding quotes from msg
	if len(msg) > 1 && msg[0] == '"' && msg[len(msg)-1] == '"' {
		msg = msg[1 : len(msg)-1]
	}
	//Default scanner from args
	readers := []io.Reader{strings.NewReader(msg + "\n")}

	if fi, _ := os.Stdin.Stat(); (fi.Mode() & os.ModeCharDevice) == 0 {
		slog.Debug(fmt.Sprintf("reading from stdin because %x\n", fi.Mode()|os.ModeCharDevice))
		readers = append(readers, os.Stdin)

	} else if msg == "" {
		slog.Warn("no input provided, exiting")
		return errorutils.NewReport("no input provided", "", errorutils.WithExitCode(1))
	}
	multireader := io.MultiReader(readers...)

	slog.Debug("scanning input")
	s := bufio.NewScanner(multireader)
	for s.Scan() {
		linemsg := s.Text()
		if linemsg == "" {
			continue
		}
		levelFunc(linemsg)
	}
	slog.Debug("finished scanning input")
	return nil
}

func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

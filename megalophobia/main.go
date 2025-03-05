package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitId string
	input    io.Reader //defaults to stdin
)

var app = cli.Command{
	Name:        "megalophobia",
	Description: "Makes a three line window to display info, input is meant to be human paced. No scrolling",
	Before: func(c context.Context, cmd *cli.Command) (context.Context, error) {
		input = os.Stdin
		return nil, nil
	},
	Action:  tool,
	Version: "v0.0.2 - commit: " + CommitId,
	Commands: []*cli.Command{
		{
			Name:        "demo",
			Description: "show example output to see behaviour",
			Action:      phobia,
		},
	},
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Enable debug logging",
			Action: func(c context.Context, cmd *cli.Command, b bool) error {
				if b {
					logrus.SetLevel(logrus.DebugLevel)
					logrus.Debug("Debug logging enabled")
					logrus.SetOutput(os.Stderr)
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Usage:   "Path to input file",
			Value:   "",
			Action: func(c context.Context, cmd *cli.Command, s string) error {
				f, err := os.Open(s)
				if err != nil {
					return err
				}
				input = f
				return nil
			},
		},
		&cli.BoolFlag{
			Name:    "pace",
			Aliases: []string{"p"},
			Usage:   "Pace input",
		},
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func tool(ctx context.Context, cmd *cli.Command) error {
	running, err := SetupCapture()
	errorutils.ExitOnFail(err, errorutils.WithMsg("Failed to setup capture: "))
	//reading loop
	scanner := bufio.NewScanner(input)
	for *running && scanner.Scan() {
		userinput := scanner.Text()
		fmt.Println(userinput)
		if cmd.Bool("pace") {
			time.Sleep(400 * time.Millisecond)
		}
	}
	FinishCapture() // blocks main goroutineCapture()
	return nil
}

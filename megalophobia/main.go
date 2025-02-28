package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitID string
)

func init() {
	if os.Getenv("DEBUG_MODE") == "true" {
		err := func(filename string) error {
			f, err := os.Open(filename)
			if err != nil {
				panic(err)
			}
			os.Stdin = f
			return nil
		}("test2.txt")
		if err != nil {
			panic(err)
		}
	}
}

var app = cli.Command{
	Name:        "megalophobia",
	Description: "Makes a three line window to display info, input is meant to be human paced. No scrolling",
	Action:      tool,
	Version:     "v0.0.1 - commit: " + CommitID,
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
				}
				return nil
			},
		},
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func tool(ctx context.Context, cmd *cli.Command) error {
	var running bool = true
	err := SetupCapture()
	go AsyncUpdateBuffer()
	errorutils.ExitOnFail(err, errorutils.WithMsg("Failed to setup capture: "))
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, syscall.SIGINT)

	go func() {
		for sig := range interrupted {
			if sig == syscall.SIGINT {
				running = false
			}
		}
	}()
	//reading loop
	var userinput string
	for running {
		_, err = fmt.Scanln(&userinput)
		if err != nil {
			fmt.Printf("\033[31mmegalophobia error: \033[0m%s", err.Error())
			fmt.Printf("input: %#v\n", userinput)
			continue
		}
		fmt.Println(userinput)
	}

	return nil
}
